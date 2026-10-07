package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"renamemusic/internal/google"
)

func TestVideoID(t *testing.T) {
	cases := map[string]string{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ":                    "dQw4w9WgXcQ",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=PLabc&index=2": "dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ?si=xyz":                            "dQw4w9WgXcQ",
		"https://m.youtube.com/watch?v=dQw4w9WgXcQ":                      "dQw4w9WgXcQ",
		"https://music.youtube.com/watch?v=dQw4w9WgXcQ&feature=share":    "dQw4w9WgXcQ",
		"https://www.youtube.com/shorts/dQw4w9WgXcQ":                     "dQw4w9WgXcQ",
		"https://www.youtube.com/playlist?list=PLabc":                    "",
		"https://example.com/watch?v=dQw4w9WgXcQ":                        "",
		"https://www.youtube.com/watch?v=corto":                          "",
		"non un link":                                                    "",
	}
	for link, want := range cases {
		if got := VideoID(link); got != want {
			t.Errorf("VideoID(%q) = %q, atteso %q", link, got, want)
		}
	}
}

func TestPlaylistID(t *testing.T) {
	cases := map[string]string{
		"https://www.youtube.com/playlist?list=PLabc123":            "PLabc123",
		"https://music.youtube.com/playlist?list=PLabc123&si=x":     "PLabc123",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=PLabc123": "PLabc123",
		"https://www.youtube.com/@canale":                           "",
		"https://example.com/playlist?list=PLabc123":                "",
	}
	for link, want := range cases {
		if got := PlaylistID(link); got != want {
			t.Errorf("PlaylistID(%q) = %q, atteso %q", link, got, want)
		}
	}
	if got := PlaylistID(PlaylistURL("PLxyz")); got != "PLxyz" {
		t.Errorf("PlaylistURL non riletto: %q", got)
	}
}

// fakeAPI simula la YouTube Data API: una playlist con due pagine di elementi.
type fakeAPI struct {
	mu       sync.Mutex
	deleted  []string
	inserted []string
	reject   int // numero di richieste da rifiutare con 401
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.reject > 0 {
		f.reject--
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if r.Header.Get("Authorization") != "Bearer tok" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/playlistItems":
		if r.URL.Query().Get("pageToken") == "" {
			io.WriteString(w, `{"nextPageToken":"p2","items":[
				{"id":"i1","snippet":{"title":"Uno","resourceId":{"videoId":"vid1"}}},
				{"id":"i2","snippet":{"title":"Due","resourceId":{"videoId":"vid2"}}}]}`)
			return
		}
		io.WriteString(w, `{"items":[{"id":"i3","snippet":{"title":"Uno bis","resourceId":{"videoId":"vid1"}}}]}`)
	case r.Method == http.MethodGet && r.URL.Path == "/playlists":
		io.WriteString(w, `{"items":[{"id":"PL1","snippet":{"title":"Coda"},"contentDetails":{"itemCount":3},"status":{"privacyStatus":"private"}}]}`)
	case r.Method == http.MethodDelete && r.URL.Path == "/playlistItems":
		id := r.URL.Query().Get("id")
		if id == "quota" {
			w.WriteHeader(http.StatusForbidden)
			io.WriteString(w, `{"error":{"code":403,"message":"quota","errors":[{"reason":"quotaExceeded"}]}}`)
			return
		}
		f.deleted = append(f.deleted, id)
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodPost && r.URL.Path == "/playlistItems":
		var body struct {
			Snippet struct {
				PlaylistID string `json:"playlistId"`
				ResourceID struct {
					VideoID string `json:"videoId"`
				} `json:"resourceId"`
			} `json:"snippet"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.inserted = append(f.inserted, body.Snippet.PlaylistID+"/"+body.Snippet.ResourceID.VideoID)
		io.WriteString(w, `{}`)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func newTestClient(t *testing.T, api *fakeAPI) (*Client, *int) {
	t.Helper()
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	old := BaseURL
	BaseURL = srv.URL
	t.Cleanup(func() { BaseURL = old })
	invalidated := 0
	return &Client{
		Token:      func(context.Context) (string, error) { return "tok", nil },
		Invalidate: func(string) { invalidated++ },
	}, &invalidated
}

func TestItemsPagingAndDelete(t *testing.T) {
	api := &fakeAPI{}
	c, _ := newTestClient(t, api)
	ctx := context.Background()

	items, err := c.Items(ctx, "PL1")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || items[2].ID != "i3" || items[2].VideoID != "vid1" {
		t.Fatalf("elementi inattesi: %+v", items)
	}
	if err := c.Delete(ctx, "i1"); err != nil {
		t.Fatal(err)
	}
	if err := c.Insert(ctx, "PL1", "vid9"); err != nil {
		t.Fatal(err)
	}
	if len(api.deleted) != 1 || api.deleted[0] != "i1" || len(api.inserted) != 1 || api.inserted[0] != "PL1/vid9" {
		t.Fatalf("operazioni inattese: deleted=%v inserted=%v", api.deleted, api.inserted)
	}

	pls, err := c.Playlists(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(pls) != 1 || pls[0].Title != "Coda" || pls[0].Count != 3 || pls[0].Privacy != "private" {
		t.Fatalf("playlist inattese: %+v", pls)
	}
}

func TestQuotaError(t *testing.T) {
	c, _ := newTestClient(t, &fakeAPI{})
	err := c.Delete(context.Background(), "quota")
	if !IsQuota(err) {
		t.Fatalf("atteso errore di quota, ottenuto %v", err)
	}
	if !strings.Contains(err.Error(), "quota") {
		t.Errorf("messaggio poco chiaro: %v", err)
	}
}

// Un 401 fa scartare il token e riprovare una volta; due di fila => ErrSignedOut.
func TestUnauthorizedRetry(t *testing.T) {
	api := &fakeAPI{reject: 1}
	c, invalidated := newTestClient(t, api)
	if _, err := c.Playlists(context.Background()); err != nil {
		t.Fatalf("il secondo tentativo doveva riuscire: %v", err)
	}
	if *invalidated != 1 {
		t.Errorf("Invalidate chiamata %d volte, attesa 1", *invalidated)
	}

	api.reject = 2
	_, err := c.Playlists(context.Background())
	if !errors.Is(err, google.ErrSignedOut) {
		t.Fatalf("atteso ErrSignedOut, ottenuto %v", err)
	}
}
