// Package drive legge e scrive file nella cartella dati dell'app su Google
// Drive (appDataFolder): una cartella nascosta, legata all'account e al
// progetto Google Cloud dell'app, invisibile fra i file dell'utente. Serve a
// condividere le impostazioni fra i dispositivi collegati allo stesso account.
// Chiamate REST dirette, come il package youtube (gira anche con gomobile).
package drive

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"time"

	"renamemusic/internal/google"
)

// Indirizzi della Drive API (variabili per i test, anche di altri package).
var (
	BaseURL   = "https://www.googleapis.com/drive/v3"
	UploadURL = "https://www.googleapis.com/upload/drive/v3"
)

// ErrScope indica un accesso senza il permesso google.DriveAppDataScope (account
// collegato prima che l'app lo chiedesse, o permesso non concesso).
var ErrScope = errors.New("manca il permesso per le impostazioni su Google Drive: scollega e ricollega l'account")

// ErrDisabled indica la Drive API non abilitata nel progetto Google Cloud.
var ErrDisabled = errors.New("la Google Drive API non è abilitata nel progetto Google Cloud (API e servizi > Libreria)")

// Client esegue le chiamate con l'access token di Token (come youtube.Client).
type Client struct {
	Token      func(ctx context.Context) (string, error)
	Invalidate func(token string)
	HTTP       *http.Client
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

// do esegue la richiesta costruita da build (con il token) e restituisce il
// corpo della risposta. Un 401 fa scartare il token e riprovare una volta.
func (c *Client) do(ctx context.Context, build func(token string) (*http.Request, error)) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		token, err := c.Token(ctx)
		if err != nil {
			return nil, err
		}
		req, err := build(token)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := c.httpClient().Do(req)
		if err != nil {
			return nil, fmt.Errorf("Google Drive non raggiungibile: %w", err)
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusUnauthorized {
			if c.Invalidate != nil {
				c.Invalidate(token)
			}
			if attempt == 0 {
				continue
			}
			return nil, google.ErrSignedOut
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return nil, apiError(resp.StatusCode, data)
		}
		return data, nil
	}
}

// apiError traduce un errore della Drive API.
func apiError(status int, data []byte) error {
	var body struct {
		Error struct {
			Message string `json:"message"`
			Errors  []struct {
				Reason string `json:"reason"`
			} `json:"errors"`
			Details []struct {
				Reason string `json:"reason"`
			} `json:"details"`
		} `json:"error"`
	}
	_ = json.Unmarshal(data, &body)
	reasons := map[string]bool{}
	for _, e := range body.Error.Errors {
		reasons[e.Reason] = true
	}
	for _, d := range body.Error.Details {
		reasons[d.Reason] = true
	}
	switch {
	case reasons["insufficientPermissions"] || reasons["ACCESS_TOKEN_SCOPE_INSUFFICIENT"]:
		return ErrScope
	case reasons["accessNotConfigured"] || reasons["SERVICE_DISABLED"]:
		return ErrDisabled
	case body.Error.Message != "":
		return fmt.Errorf("errore di Google Drive (HTTP %d): %s", status, body.Error.Message)
	}
	return fmt.Errorf("errore di Google Drive (HTTP %d)", status)
}

// findAppFile restituisce l'ID del file name nella cartella dati dell'app
// ("" se non c'è). Se due dispositivi l'hanno creato nello stesso momento ce
// ne sono più copie: vale sempre la più vecchia, e le altre si eliminano
// (un dispositivo che ci aveva scritto ricarica le sue impostazioni, più
// recenti di quelle della copia buona).
func (c *Client) findAppFile(ctx context.Context, name string) (string, error) {
	q := url.Values{
		"spaces":   {"appDataFolder"},
		"q":        {fmt.Sprintf("name = '%s' and trashed = false", name)},
		"fields":   {"files(id)"},
		"orderBy":  {"createdTime"},
		"pageSize": {"10"},
	}
	data, err := c.do(ctx, func(string) (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodGet, BaseURL+"/files?"+q.Encode(), nil)
	})
	if err != nil {
		return "", err
	}
	var out struct {
		Files []struct {
			ID string `json:"id"`
		} `json:"files"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("risposta di Google Drive non valida: %w", err)
	}
	if len(out.Files) == 0 {
		return "", nil
	}
	for _, extra := range out.Files[1:] {
		_, _ = c.do(ctx, func(string) (*http.Request, error) {
			return http.NewRequestWithContext(ctx, http.MethodDelete, BaseURL+"/files/"+url.PathEscape(extra.ID), nil)
		})
	}
	return out.Files[0].ID, nil
}

// ReadAppFile legge il file name della cartella dati dell'app; found è false
// se non esiste ancora.
func (c *Client) ReadAppFile(ctx context.Context, name string) (data []byte, found bool, err error) {
	id, err := c.findAppFile(ctx, name)
	if err != nil || id == "" {
		return nil, false, err
	}
	data, err = c.do(ctx, func(string) (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodGet, BaseURL+"/files/"+url.PathEscape(id)+"?alt=media", nil)
	})
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// WriteAppFile scrive (crea o sostituisce) il file JSON name nella cartella
// dati dell'app.
func (c *Client) WriteAppFile(ctx context.Context, name string, content []byte) error {
	id, err := c.findAppFile(ctx, name)
	if err != nil {
		return err
	}
	if id != "" {
		_, err = c.do(ctx, func(string) (*http.Request, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodPatch,
				UploadURL+"/files/"+url.PathEscape(id)+"?uploadType=media", bytes.NewReader(content))
			if err == nil {
				req.Header.Set("Content-Type", "application/json")
			}
			return req, err
		})
		return err
	}

	// Creazione: metadati (nome e cartella) e contenuto in un'unica
	// richiesta multipart/related.
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	meta, _ := json.Marshal(map[string]any{"name": name, "parents": []string{"appDataFolder"}})
	for _, part := range [][]byte{meta, content} {
		w, err := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/json; charset=UTF-8"}})
		if err != nil {
			return err
		}
		if _, err := w.Write(part); err != nil {
			return err
		}
	}
	if err := mw.Close(); err != nil {
		return err
	}
	payload := body.Bytes()
	_, err = c.do(ctx, func(string) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, UploadURL+"/files?uploadType=multipart", bytes.NewReader(payload))
		if err == nil {
			req.Header.Set("Content-Type", "multipart/related; boundary="+mw.Boundary())
		}
		return req, err
	})
	return err
}
