// Package google gestisce l'accesso all'account Google dell'utente (OAuth 2.0)
// per la YouTube Data API. Su desktop il flusso è quello delle app installate:
// il browser di sistema apre la pagina di consenso di Google, che rimanda a un
// piccolo server locale su 127.0.0.1 (DesktopAuth). Su Android l'accesso passa
// invece da Google Play Services (package mobile); comune a entrambi è solo la
// revoca (Revoke).
package google

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// Permessi chiesti all'utente: Scope è la gestione del proprio account
// YouTube (lettura e modifica delle playlist), EmailScope l'indirizzo email
// dell'account, per mostrare con quale account si è collegati,
// DriveAppDataScope la sola cartella dati nascosta dell'app su Google Drive,
// dove si condividono le impostazioni fra i dispositivi (package drive).
// Solo Scope è indispensabile.
const (
	Scope             = "https://www.googleapis.com/auth/youtube"
	EmailScope        = "https://www.googleapis.com/auth/userinfo.email"
	DriveAppDataScope = "https://www.googleapis.com/auth/drive.appdata"
)

// userinfoURL restituisce i dati dell'account dell'access token (variabile per i test).
var userinfoURL = "https://openidconnect.googleapis.com/v1/userinfo"

// UserEmail restituisce l'indirizzo email dell'account a cui appartiene token
// (serve il permesso EmailScope).
func UserEmail(ctx context.Context, token string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userinfoURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("lettura dell'account fallita: HTTP %d", resp.StatusCode)
	}
	var info struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", err
	}
	return info.Email, nil
}

// ErrSignedOut indica che non c'è un account collegato, o che l'accesso non è
// più valido (revocato dall'utente, scaduto): serve un nuovo SignIn.
var ErrSignedOut = errors.New("account Google non collegato")

// ErrScopeDenied indica un accesso in cui l'utente non ha concesso il
// permesso su YouTube (la casella nella pagina di consenso di Google).
var ErrScopeDenied = errors.New("non hai concesso l'accesso a YouTube: ricollega l'account e, nella pagina di Google, spunta la casella «Gestisci il tuo account YouTube»")

// ErrNotConfigured indica una build senza le credenziali OAuth del client
// desktop (vedi credentials.go).
var ErrNotConfigured = errors.New("accesso a Google non configurato in questa versione dell'app")

// Endpoint OAuth di Google (variabili per i test).
var (
	authURL   = "https://accounts.google.com/o/oauth2/v2/auth"
	tokenURL  = "https://oauth2.googleapis.com/token"
	revokeURL = "https://oauth2.googleapis.com/revoke"
)

// signInTimeout limita l'attesa del ritorno dal browser: chi chiude la pagina
// senza completare l'accesso non lascia il server locale aperto per sempre.
const signInTimeout = 5 * time.Minute

// expiryMargin: un access token si rinnova un po' prima della scadenza, così
// non scade a metà di una serie di richieste.
const expiryMargin = time.Minute

var httpClient = &http.Client{Timeout: 30 * time.Second}

// Revoke revoca l'accesso concesso all'app: vale sia per un access token sia
// per un refresh token, e toglie l'intero consenso (l'app sparisce da
// "App con accesso al tuo account").
func Revoke(ctx context.Context, token string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, revokeURL, strings.NewReader(url.Values{"token": {token}}.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// 400 = token già non valido: per chi si scollega va bene lo stesso.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		return fmt.Errorf("revoca dell'accesso fallita: HTTP %d", resp.StatusCode)
	}
	return nil
}

// tokenResponse è la risposta dell'endpoint token (anche in caso di errore).
type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshToken     string `json:"refresh_token"`
	Scope            string `json:"scope"` // permessi concessi, separati da spazi
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func postToken(ctx context.Context, form url.Values) (tokenResponse, error) {
	var tr tokenResponse
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return tr, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient.Do(req)
	if err != nil {
		return tr, err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return tr, fmt.Errorf("risposta di Google non valida (HTTP %d)", resp.StatusCode)
	}
	if tr.Error != "" {
		// invalid_grant: refresh token revocato, scaduto (7 giorni con l'app
		// in modalità "Testing") o codice già usato.
		if tr.Error == "invalid_grant" {
			return tr, ErrSignedOut
		}
		msg := tr.Error
		if tr.ErrorDescription != "" {
			msg += ": " + tr.ErrorDescription
		}
		return tr, fmt.Errorf("accesso a Google rifiutato (%s)", msg)
	}
	if resp.StatusCode != http.StatusOK || tr.AccessToken == "" {
		return tr, fmt.Errorf("accesso a Google fallito: HTTP %d", resp.StatusCode)
	}
	return tr, nil
}

// DesktopAuth è l'accesso a Google su desktop. Il refresh token si conserva in
// path, cifrato per l'utente di Windows (DPAPI, vedi protect_windows.go);
// l'access token (valido un'ora) resta solo in memoria.
type DesktopAuth struct {
	path        string
	openBrowser func(url string) error

	mu      sync.Mutex
	refresh string
	access  string
	expiry  time.Time
}

// storedToken è il contenuto (prima della cifratura) del file del token.
type storedToken struct {
	RefreshToken string `json:"refreshToken"`
}

// NewDesktopAuth crea l'accesso desktop con il refresh token salvato in path
// (se c'è); openBrowser apre un indirizzo nel browser di sistema.
func NewDesktopAuth(path string, openBrowser func(url string) error) *DesktopAuth {
	d := &DesktopAuth{path: path, openBrowser: openBrowser}
	if data, err := os.ReadFile(path); err == nil {
		if plain, err := unprotect(data); err == nil {
			var st storedToken
			if json.Unmarshal(plain, &st) == nil {
				d.refresh = st.RefreshToken
			}
		}
	}
	return d
}

// Configured indica se la build ha le credenziali del client OAuth.
func (d *DesktopAuth) Configured() bool {
	id, _ := credentials()
	return id != ""
}

// SignIn apre nel browser la pagina di consenso di Google e attende il
// ritorno sul server locale (al massimo signInTimeout, o fino ad annullamento
// di ctx), poi scambia il codice ottenuto con i token e salva il refresh token.
func (d *DesktopAuth) SignIn(ctx context.Context) error {
	id, secret := credentials()
	if id == "" {
		return ErrNotConfigured
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("impossibile avviare il server locale per l'accesso: %w", err)
	}
	defer ln.Close()
	redirect := fmt.Sprintf("http://127.0.0.1:%d/", ln.Addr().(*net.TCPAddr).Port)

	state := randomString(24)
	verifier := randomString(48)
	challenge := sha256.Sum256([]byte(verifier))

	type callback struct {
		code string
		err  error
	}
	done := make(chan callback, 1)
	srv := &http.Server{
		ReadHeaderTimeout: 10 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			q := r.URL.Query()
			var cb callback
			switch {
			case q.Get("state") != state:
				cb.err = errors.New("risposta di Google non valida (state diverso)")
			case q.Get("error") == "access_denied":
				cb.err = errors.New("accesso negato: non hai concesso il permesso")
			case q.Get("error") != "":
				cb.err = fmt.Errorf("accesso a Google fallito (%s)", q.Get("error"))
			case q.Get("code") == "":
				cb.err = errors.New("risposta di Google senza codice di accesso")
			default:
				cb.code = q.Get("code")
			}
			writeCallbackPage(w, cb.err)
			select {
			case done <- cb:
			default: // già ricevuto (es. pagina ricaricata)
			}
		}),
	}
	go srv.Serve(ln)
	defer srv.Close()

	consent := authURL + "?" + url.Values{
		"client_id":             {id},
		"redirect_uri":          {redirect},
		"response_type":         {"code"},
		"scope":                 {Scope + " " + EmailScope + " " + DriveAppDataScope},
		"state":                 {state},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"code_challenge_method": {"S256"},
		// offline + consent: Google restituisce sempre il refresh token, anche
		// se l'utente aveva già dato il consenso in passato.
		"access_type": {"offline"},
		"prompt":      {"consent"},
	}.Encode()
	if err := d.openBrowser(consent); err != nil {
		return fmt.Errorf("impossibile aprire il browser: %w", err)
	}

	timer := time.NewTimer(signInTimeout)
	defer timer.Stop()
	var cb callback
	select {
	case cb = <-done:
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return errors.New("tempo scaduto: l'accesso non è stato completato nel browser")
	}
	if cb.err != nil {
		return cb.err
	}

	tr, err := postToken(ctx, url.Values{
		"client_id":     {id},
		"client_secret": {secret},
		"code":          {cb.code},
		"code_verifier": {verifier},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {redirect},
	})
	if errors.Is(err, ErrSignedOut) {
		return errors.New("codice di accesso non valido: riprova")
	}
	if err != nil {
		return err
	}
	if tr.RefreshToken == "" {
		return errors.New("Google non ha restituito il token di accesso permanente: riprova")
	}
	// Nella pagina di consenso ogni permesso ha la sua casella: chi lascia
	// senza spunta quello di YouTube concede un accesso inutile all'app.
	if tr.Scope != "" && !slices.Contains(strings.Fields(tr.Scope), Scope) {
		_ = Revoke(ctx, tr.RefreshToken)
		return ErrScopeDenied
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.refresh = tr.RefreshToken
	d.access = tr.AccessToken
	d.expiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	return d.saveLocked()
}

// Token restituisce un access token valido, rinnovandolo con il refresh token
// se è scaduto. ErrSignedOut se non c'è un account collegato o se Google ha
// rifiutato il rinnovo (in quel caso il refresh token salvato si elimina).
func (d *DesktopAuth) Token(ctx context.Context) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.refresh == "" {
		return "", ErrSignedOut
	}
	if d.access != "" && time.Now().Add(expiryMargin).Before(d.expiry) {
		return d.access, nil
	}
	id, secret := credentials()
	if id == "" {
		return "", ErrNotConfigured
	}
	tr, err := postToken(ctx, url.Values{
		"client_id":     {id},
		"client_secret": {secret},
		"refresh_token": {d.refresh},
		"grant_type":    {"refresh_token"},
	})
	if errors.Is(err, ErrSignedOut) {
		d.clearLocked()
		return "", ErrSignedOut
	}
	if err != nil {
		return "", err
	}
	d.access = tr.AccessToken
	d.expiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	return d.access, nil
}

// Invalidate scarta l'access token in memoria (rifiutato dall'API): il
// prossimo Token lo rinnova.
func (d *DesktopAuth) Invalidate(token string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.access == token {
		d.access = ""
	}
}

// SignOut revoca l'accesso su Google (se possibile: anche offline il token
// locale si elimina comunque) e dimentica i token.
func (d *DesktopAuth) SignOut(ctx context.Context) error {
	d.mu.Lock()
	refresh := d.refresh
	d.clearLocked()
	d.mu.Unlock()
	if refresh == "" {
		return nil
	}
	return Revoke(ctx, refresh)
}

func (d *DesktopAuth) clearLocked() {
	d.refresh, d.access, d.expiry = "", "", time.Time{}
	_ = os.Remove(d.path)
}

func (d *DesktopAuth) saveLocked() error {
	plain, err := json.Marshal(storedToken{RefreshToken: d.refresh})
	if err != nil {
		return err
	}
	data, err := protect(plain)
	if err != nil {
		return fmt.Errorf("impossibile cifrare il token di Google: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(d.path), 0o755); err != nil {
		return err
	}
	tmp := d.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, d.path); err != nil {
		_ = os.Remove(d.path)
		if err := os.Rename(tmp, d.path); err != nil {
			_ = os.Remove(tmp)
			return err
		}
	}
	return nil
}

// randomString restituisce n byte casuali in base64 URL (state e verifier PKCE).
func randomString(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// writeCallbackPage è la pagina mostrata nel browser al ritorno da Google.
func writeCallbackPage(w http.ResponseWriter, err error) {
	title, text := "Accesso completato", "Puoi chiudere questa scheda e tornare a RenameMusic."
	if err != nil {
		title, text = "Accesso non riuscito", html.EscapeString(err.Error())+". Torna a RenameMusic per riprovare."
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!doctype html><html lang="it"><head><meta charset="utf-8"><title>RenameMusic</title>
<style>body{font-family:system-ui,sans-serif;background:#0f172a;color:#e2e8f0;display:grid;place-items:center;min-height:100vh;margin:0}
main{max-width:420px;padding:32px;border-radius:16px;background:#1e293b;text-align:center}h1{font-size:20px;margin:0 0 12px}p{margin:0;color:#94a3b8;line-height:1.5}</style>
</head><body><main><h1>%s</h1><p>%s</p></main></body></html>`, title, text)
}
