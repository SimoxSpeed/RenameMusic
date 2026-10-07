// Package youtube è un client minimo della YouTube Data API v3, limitato a ciò
// che serve all'app: le playlist dell'account collegato, i loro elementi,
// l'aggiunta e la rimozione di un video. Chiamate REST dirette (niente librerie
// di Google): il package gira anche nel core Android compilato con gomobile.
//
// Quota: ogni progetto Google Cloud ha 10.000 unità al giorno; una lettura
// costa 1 unità (fino a 50 elementi), un'aggiunta o una rimozione 50.
package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"renamemusic/internal/google"
)

// BaseURL è l'indirizzo della YouTube Data API (variabile per i test, anche
// di altri package).
var BaseURL = "https://www.googleapis.com/youtube/v3"

// Client esegue le chiamate con l'access token di Token. Invalidate (può essere
// nil) scarta un token rifiutato dall'API: Client riprova una volta con uno
// nuovo prima di arrendersi.
type Client struct {
	Token      func(ctx context.Context) (string, error)
	Invalidate func(token string)
	HTTP       *http.Client
}

// Playlist è una playlist dell'account collegato.
type Playlist struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Count   int    `json:"count"`
	Privacy string `json:"privacy"` // public | unlisted | private
}

// Item è un elemento di una playlist. ID identifica l'elemento (serve per
// rimuoverlo), VideoID il video: lo stesso video può comparire più volte.
type Item struct {
	ID      string
	VideoID string
	Title   string
}

// APIError è un errore restituito dall'API, con il motivo ("reason") di Google.
type APIError struct {
	Status  int
	Reason  string
	Message string
}

func (e *APIError) Error() string {
	switch e.Reason {
	case "quotaExceeded", "dailyLimitExceeded", "rateLimitExceeded", "userRateLimitExceeded":
		return "quota giornaliera della YouTube Data API esaurita: riprova domani"
	case "playlistNotFound":
		return "playlist non trovata sull'account collegato"
	case "insufficientPermissions", "ACCESS_TOKEN_SCOPE_INSUFFICIENT":
		return "l'app non ha il permesso di accedere a YouTube: scollega l'account, ricollegalo e nella pagina di Google spunta «Gestisci il tuo account YouTube»"
	case "accessNotConfigured", "SERVICE_DISABLED":
		return "la YouTube Data API non è abilitata nel progetto Google Cloud (API e servizi > Libreria)"
	case "playlistItemsNotAccessible", "playlistForbidden":
		return "operazione non consentita: la playlist deve appartenere all'account collegato"
	case "videoNotFound":
		return "video non trovato su YouTube"
	case "youtubeSignupRequired":
		return "l'account Google non ha un canale YouTube"
	case "playlistContainsMaximumNumberOfVideos":
		return "la playlist ha già il numero massimo di video"
	}
	if e.Status == http.StatusNotFound {
		return "elemento non trovato su YouTube"
	}
	if e.Message != "" && e.Reason != "" {
		return fmt.Sprintf("errore di YouTube (HTTP %d, %s): %s", e.Status, e.Reason, e.Message)
	}
	if e.Message != "" {
		return fmt.Sprintf("errore di YouTube (HTTP %d): %s", e.Status, e.Message)
	}
	return fmt.Sprintf("errore di YouTube (HTTP %d)", e.Status)
}

// IsQuota indica un errore di quota esaurita: inutile proseguire con altre
// chiamate fino al giorno dopo.
func IsQuota(err error) bool {
	var e *APIError
	if !errors.As(err, &e) {
		return false
	}
	switch e.Reason {
	case "quotaExceeded", "dailyLimitExceeded", "rateLimitExceeded", "userRateLimitExceeded":
		return true
	}
	return false
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

// do esegue una richiesta all'API (path relativo a BaseURL, con la query) e
// decodifica la risposta in out (se non nil). Un 401 fa scartare il token e
// riprovare una volta; se Google rifiuta anche il nuovo, ErrSignedOut.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return err
		}
	}
	for attempt := 0; ; attempt++ {
		token, err := c.Token(ctx)
		if err != nil {
			return err
		}
		u := BaseURL + path
		if len(query) > 0 {
			u += "?" + query.Encode()
		}
		req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := c.httpClient().Do(req)
		if err != nil {
			return fmt.Errorf("YouTube non raggiungibile: %w", err)
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if err != nil {
			return err
		}
		if resp.StatusCode == http.StatusUnauthorized {
			if c.Invalidate != nil {
				c.Invalidate(token)
			}
			if attempt == 0 {
				continue
			}
			return google.ErrSignedOut
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return parseError(resp.StatusCode, data)
		}
		if out == nil || len(data) == 0 {
			return nil
		}
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("risposta di YouTube non valida: %w", err)
		}
		return nil
	}
}

func parseError(status int, data []byte) error {
	var body struct {
		Error struct {
			Message string `json:"message"`
			Errors  []struct {
				Reason string `json:"reason"`
			} `json:"errors"`
			// Le API più recenti mettono il motivo preciso (es.
			// ACCESS_TOKEN_SCOPE_INSUFFICIENT) in un ErrorInfo fra i dettagli.
			Details []struct {
				Reason string `json:"reason"`
			} `json:"details"`
		} `json:"error"`
	}
	e := &APIError{Status: status}
	if json.Unmarshal(data, &body) == nil {
		e.Message = body.Error.Message
		if len(body.Error.Errors) > 0 {
			e.Reason = body.Error.Errors[0].Reason
		}
		if e.Reason == "" || e.Reason == "forbidden" {
			for _, d := range body.Error.Details {
				if d.Reason != "" {
					e.Reason = d.Reason
					break
				}
			}
		}
	}
	return e
}

// Playlists restituisce tutte le playlist dell'account collegato.
func (c *Client) Playlists(ctx context.Context) ([]Playlist, error) {
	var list []Playlist
	page := ""
	for {
		var out struct {
			NextPageToken string `json:"nextPageToken"`
			Items         []struct {
				ID      string `json:"id"`
				Snippet struct {
					Title string `json:"title"`
				} `json:"snippet"`
				ContentDetails struct {
					ItemCount int `json:"itemCount"`
				} `json:"contentDetails"`
				Status struct {
					PrivacyStatus string `json:"privacyStatus"`
				} `json:"status"`
			} `json:"items"`
		}
		q := url.Values{"part": {"snippet,contentDetails,status"}, "mine": {"true"}, "maxResults": {"50"}}
		if page != "" {
			q.Set("pageToken", page)
		}
		if err := c.do(ctx, http.MethodGet, "/playlists", q, nil, &out); err != nil {
			return nil, err
		}
		for _, it := range out.Items {
			list = append(list, Playlist{
				ID:      it.ID,
				Title:   it.Snippet.Title,
				Count:   it.ContentDetails.ItemCount,
				Privacy: it.Status.PrivacyStatus,
			})
		}
		if out.NextPageToken == "" {
			return list, nil
		}
		page = out.NextPageToken
	}
}

// Items restituisce tutti gli elementi della playlist playlistID.
func (c *Client) Items(ctx context.Context, playlistID string) ([]Item, error) {
	var items []Item
	page := ""
	for {
		var out struct {
			NextPageToken string `json:"nextPageToken"`
			Items         []struct {
				ID      string `json:"id"`
				Snippet struct {
					Title      string `json:"title"`
					ResourceID struct {
						VideoID string `json:"videoId"`
					} `json:"resourceId"`
				} `json:"snippet"`
			} `json:"items"`
		}
		q := url.Values{"part": {"snippet"}, "playlistId": {playlistID}, "maxResults": {"50"}}
		if page != "" {
			q.Set("pageToken", page)
		}
		if err := c.do(ctx, http.MethodGet, "/playlistItems", q, nil, &out); err != nil {
			return nil, err
		}
		for _, it := range out.Items {
			items = append(items, Item{ID: it.ID, VideoID: it.Snippet.ResourceID.VideoID, Title: it.Snippet.Title})
		}
		if out.NextPageToken == "" {
			return items, nil
		}
		page = out.NextPageToken
	}
}

// Insert aggiunge il video videoID in fondo alla playlist playlistID.
func (c *Client) Insert(ctx context.Context, playlistID, videoID string) error {
	body := map[string]any{
		"snippet": map[string]any{
			"playlistId": playlistID,
			"resourceId": map[string]string{"kind": "youtube#video", "videoId": videoID},
		},
	}
	return c.do(ctx, http.MethodPost, "/playlistItems", url.Values{"part": {"snippet"}}, body, nil)
}

// Delete rimuove l'elemento itemID (Item.ID) dalla sua playlist.
func (c *Client) Delete(ctx context.Context, itemID string) error {
	return c.do(ctx, http.MethodDelete, "/playlistItems", url.Values{"id": {itemID}}, nil, nil)
}

// PlaylistURL è il link della playlist id.
func PlaylistURL(id string) string {
	return "https://www.youtube.com/playlist?list=" + url.QueryEscape(id)
}

// isYouTubeHost indica un indirizzo di YouTube (anche m., music., youtu.be).
func isYouTubeHost(host string) bool {
	host = strings.ToLower(strings.TrimPrefix(host, "www."))
	return host == "youtube.com" || strings.HasSuffix(host, ".youtube.com") || host == "youtu.be"
}

// PlaylistID estrae l'ID della playlist da un link di YouTube (parametro
// "list"), "" se il link non ne ha uno.
func PlaylistID(link string) string {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil || !isYouTubeHost(u.Hostname()) {
		return ""
	}
	return u.Query().Get("list")
}

var videoIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// VideoID estrae l'ID del video da un link di YouTube (watch?v=, youtu.be/,
// shorts/, live/, embed/), "" se il link non punta a un video.
func VideoID(link string) string {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil || !isYouTubeHost(u.Hostname()) {
		return ""
	}
	id := ""
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	switch {
	case strings.EqualFold(u.Hostname(), "youtu.be"):
		id = parts[0]
	case u.Query().Get("v") != "":
		id = u.Query().Get("v")
	case len(parts) >= 2 && (parts[0] == "shorts" || parts[0] == "live" || parts[0] == "embed"):
		id = parts[1]
	}
	if !videoIDPattern.MatchString(id) {
		return ""
	}
	return id
}
