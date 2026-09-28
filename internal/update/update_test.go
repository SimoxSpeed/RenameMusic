package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"1.0.1", "1.0.0", true},
		{"v1.1.0", "1.0.9", true},
		{"2.0", "1.9.9", true},
		{"1.10.0", "1.9.0", true},
		{"1.0.0", "1.0.0", false},
		{"1.0.0", "1.0.1", false},
		{"1.2.0-beta", "1.1.0", true},
		{"latest", "1.0.0", false},
		{"", "1.0.0", false},
	}
	for _, c := range cases {
		if got := Newer(c.latest, c.current); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

// fakeGitHub simula l'API delle release e il download dell'allegato.
func fakeGitHub(t *testing.T, payload []byte, digest string, status int) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			if status != http.StatusOK {
				w.WriteHeader(status)
				return
			}
			fmt.Fprintf(w, `{"tag_name":"v9.8.7","body":" Novità ","assets":[
				{"name":"RenameMusic.exe","browser_download_url":"%s/file","size":%d,"digest":%q}]}`,
				srv.URL, len(payload), digest)
		case "/file":
			w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	prev := latestURL
	latestURL = srv.URL + "/latest"
	t.Cleanup(func() { latestURL = prev })
	return srv
}

func sha(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func TestLatestAndDownload(t *testing.T) {
	payload := []byte("nuovo eseguibile")
	fakeGitHub(t, payload, sha(payload), http.StatusOK)

	rel, err := Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rel.Version != "9.8.7" || rel.Notes != "Novità" {
		t.Fatalf("release inattesa: %+v", rel)
	}
	asset, ok := rel.Asset("renamemusic.exe")
	if !ok {
		t.Fatal("allegato non trovato")
	}

	dest := filepath.Join(t.TempDir(), "sub", "RenameMusic.exe.new")
	var last int64
	err = Download(context.Background(), asset, dest, func(_ string, done, _ int64) { last = done })
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != string(payload) || last != int64(len(payload)) {
		t.Fatalf("contenuto %q, avanzamento %d", got, last)
	}
	if _, err := os.Stat(dest + ".part"); !os.IsNotExist(err) {
		t.Fatal("il file temporaneo non è stato rimosso")
	}
}

func TestDownloadRejectsBadDigest(t *testing.T) {
	payload := []byte("contenuto")
	fakeGitHub(t, payload, sha([]byte("altro")), http.StatusOK)

	rel, err := Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	asset, _ := rel.Asset("RenameMusic.exe")
	dest := filepath.Join(t.TempDir(), "app.new")
	if err := Download(context.Background(), asset, dest, nil); err == nil {
		t.Fatal("atteso errore di impronta")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("un file corrotto non deve restare al posto giusto")
	}
}

func TestLatestNoRelease(t *testing.T) {
	fakeGitHub(t, nil, "", http.StatusNotFound)
	if _, err := Latest(context.Background()); !errors.Is(err, ErrNoRelease) {
		t.Fatalf("atteso ErrNoRelease, ottenuto %v", err)
	}
}

func TestReplaceExecutable(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "RenameMusic.exe")
	os.WriteFile(exe, []byte("vecchio"), 0o755)
	os.WriteFile(exe+NewSuffix, []byte("nuovo"), 0o755)

	if err := ReplaceExecutable(exe, exe+NewSuffix); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "nuovo" {
		t.Fatalf("eseguibile = %q", got)
	}
	if got, _ := os.ReadFile(exe + OldSuffix); string(got) != "vecchio" {
		t.Fatalf("copia precedente = %q", got)
	}

	CleanupExecutable(exe)
	if _, err := os.Stat(exe + OldSuffix); !os.IsNotExist(err) {
		t.Fatal("la versione precedente non è stata rimossa")
	}
}

func TestReplaceExecutableRestoresOnError(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "RenameMusic.exe")
	os.WriteFile(exe, []byte("vecchio"), 0o755)

	if err := ReplaceExecutable(exe, filepath.Join(dir, "mancante")); err == nil {
		t.Fatal("atteso errore")
	}
	if got, _ := os.ReadFile(exe); string(got) != "vecchio" {
		t.Fatalf("l'eseguibile originale non è stato ripristinato: %q", got)
	}
}
