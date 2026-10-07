package google

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// fakeGoogle simula l'endpoint token: verifica il PKCE dello scambio del
// codice e rinnova l'access token finché il refresh token è valido.
type fakeGoogle struct {
	mu        sync.Mutex
	challenge string
	refreshes int
	revoked   bool
	scope     string // permessi concessi restituiti allo scambio del codice
}

func (f *fakeGoogle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_ = r.ParseForm()
	w.Header().Set("Content-Type", "application/json")
	if r.URL.Path == "/revoke" {
		f.revoked = true
		return
	}
	switch r.Form.Get("grant_type") {
	case "authorization_code":
		sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if r.Form.Get("code") != "codice" || base64.RawURLEncoding.EncodeToString(sum[:]) != f.challenge {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"access_token": "a1", "expires_in": 3600, "refresh_token": "r1", "scope": f.scope})
	case "refresh_token":
		if f.revoked || r.Form.Get("refresh_token") != "r1" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
			return
		}
		f.refreshes++
		json.NewEncoder(w).Encode(map[string]any{"access_token": "a2", "expires_in": 3600})
	}
}

func TestDesktopSignInAndRefresh(t *testing.T) {
	fake := &fakeGoogle{}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	oldToken, oldRevoke, oldID, oldSecret := tokenURL, revokeURL, clientID, clientSecret
	tokenURL, revokeURL, clientID, clientSecret = srv.URL+"/token", srv.URL+"/revoke", "id", "secret"
	defer func() { tokenURL, revokeURL, clientID, clientSecret = oldToken, oldRevoke, oldID, oldSecret }()

	path := filepath.Join(t.TempDir(), "google-token.dat")
	// Il "browser" segue subito il redirect con il codice, come farebbe Google
	// dopo il consenso.
	browser := func(consent string) error {
		u, err := url.Parse(consent)
		if err != nil {
			return err
		}
		q := u.Query()
		fake.mu.Lock()
		fake.challenge = q.Get("code_challenge")
		fake.mu.Unlock()
		go func() {
			resp, err := http.Get(q.Get("redirect_uri") + "?" + url.Values{"code": {"codice"}, "state": {q.Get("state")}}.Encode())
			if err == nil {
				resp.Body.Close()
			}
		}()
		return nil
	}

	d := NewDesktopAuth(path, browser)
	if !d.Configured() {
		t.Fatal("credenziali non viste")
	}
	if _, err := d.Token(context.Background()); !errors.Is(err, ErrSignedOut) {
		t.Fatalf("prima dell'accesso atteso ErrSignedOut, ottenuto %v", err)
	}
	if err := d.SignIn(context.Background()); err != nil {
		t.Fatal(err)
	}
	if tok, err := d.Token(context.Background()); err != nil || tok != "a1" {
		t.Fatalf("token %q, %v", tok, err)
	}

	// Un'istanza nuova (riavvio dell'app) rilegge il refresh token dal file e
	// rinnova l'access token.
	d2 := NewDesktopAuth(path, browser)
	if tok, err := d2.Token(context.Background()); err != nil || tok != "a2" {
		t.Fatalf("token dopo il riavvio %q, %v", tok, err)
	}
	// Token scaduto => rinnovo.
	d2.mu.Lock()
	d2.expiry = time.Now()
	d2.mu.Unlock()
	if _, err := d2.Token(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.refreshes != 2 {
		t.Errorf("rinnovi %d, attesi 2", fake.refreshes)
	}

	// Revoca lato Google: il rinnovo fallisce, il token locale si elimina.
	fake.mu.Lock()
	fake.revoked = true
	fake.mu.Unlock()
	d2.Invalidate("a2")
	if _, err := d2.Token(context.Background()); !errors.Is(err, ErrSignedOut) {
		t.Fatalf("atteso ErrSignedOut dopo la revoca, ottenuto %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("il file del token doveva essere eliminato: %v", err)
	}
}

func TestSignInCanceled(t *testing.T) {
	oldID := clientID
	clientID = "id"
	defer func() { clientID = oldID }()

	d := NewDesktopAuth(filepath.Join(t.TempDir(), "t.dat"), func(string) error { return nil })
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	if err := d.SignIn(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("atteso context.Canceled, ottenuto %v", err)
	}
}

// Se nella pagina di consenso l'utente non spunta il permesso su YouTube,
// l'accesso fallisce invece di collegare un account inutilizzabile.
func TestDesktopSignInScopeDenied(t *testing.T) {
	fake := &fakeGoogle{scope: EmailScope + " openid"}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	oldToken, oldRevoke, oldID, oldSecret := tokenURL, revokeURL, clientID, clientSecret
	tokenURL, revokeURL, clientID, clientSecret = srv.URL+"/token", srv.URL+"/revoke", "id", "secret"
	defer func() { tokenURL, revokeURL, clientID, clientSecret = oldToken, oldRevoke, oldID, oldSecret }()

	path := filepath.Join(t.TempDir(), "google-token.dat")
	d := NewDesktopAuth(path, func(consent string) error {
		u, _ := url.Parse(consent)
		q := u.Query()
		fake.mu.Lock()
		fake.challenge = q.Get("code_challenge")
		fake.mu.Unlock()
		go func() {
			if resp, err := http.Get(q.Get("redirect_uri") + "?" + url.Values{"code": {"codice"}, "state": {q.Get("state")}}.Encode()); err == nil {
				resp.Body.Close()
			}
		}()
		return nil
	})
	if err := d.SignIn(context.Background()); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("atteso ErrScopeDenied, ottenuto %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("con il permesso negato non doveva salvare il token")
	}
	if !fake.revoked {
		t.Error("l'accesso parziale doveva essere revocato")
	}
}
