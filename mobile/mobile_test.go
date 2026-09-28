package mobile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fakeHost simula il lato Kotlin: registra gli eventi e risponde a yt-dlp
// --version come farebbe youtubedl-android.
type fakeHost struct {
	mu     sync.Mutex
	events []string
}

func (h *fakeHost) Emit(event, payloadJSON string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, event)
}

func (h *fakeHost) YtDlpRun(argsJSON string) string {
	var args []string
	_ = json.Unmarshal([]byte(argsJSON), &args)
	if len(args) == 1 && args[0] == "--version" {
		return `{"stdout":"2026.01.01\n","stderr":"","error":""}`
	}
	return `{"stdout":"","stderr":"ERROR: non supportato nel test","error":"exit 1"}`
}

func (h *fakeHost) YtDlpReady() bool    { return true }
func (h *fakeHost) YtDlpUpdate() string { return "" }
func (h *fakeHost) InstallApk(string) string { return "" }

// TestCallDispatch avvia il core con un Host finto e verifica che Call invochi
// i metodi per nome con argomenti/risultati JSON, come fa il plugin Kotlin.
func TestCallDispatch(t *testing.T) {
	host := &fakeHost{}
	checkUpdates = false
	if err := Start(t.TempDir(), host); err != nil {
		t.Fatal(err)
	}

	music := t.TempDir()
	if err := os.WriteFile(filepath.Join(music, "Artista - Titolo (Official Video).mp3"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(music, "Sotto"), 0o755); err != nil {
		t.Fatal(err)
	}

	args, _ := json.Marshal([]any{music})
	if _, err := Call("SetFolder", string(args)); err != nil {
		t.Fatal(err)
	}
	out, err := Call("Scan", "[]")
	if err != nil {
		t.Fatal(err)
	}
	var resp struct {
		OK    bool `json:"ok"`
		State struct {
			Folder string `json:"folder"`
			Files  []struct {
				Preview string `json:"preview"`
			} `json:"files"`
		} `json:"state"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.OK || resp.State.Folder != music || len(resp.State.Files) != 1 {
		t.Fatalf("scan inatteso: %s", out)
	}
	if got := resp.State.Files[0].Preview; got != "Artista - Titolo.mp3" {
		t.Fatalf("anteprima attesa %q, ottenuta %q", "Artista - Titolo.mp3", got)
	}

	// Metodi di folderAPI raggiungibili con lo stesso dispatcher.
	out, err = Call("ListDirectory", string(args))
	if err != nil {
		t.Fatal(err)
	}
	var listing FolderListing
	if err := json.Unmarshal([]byte(out), &listing); err != nil {
		t.Fatal(err)
	}
	if len(listing.Dirs) != 1 || listing.Dirs[0] != "Sotto" {
		t.Fatalf("sottocartelle attese [Sotto], ottenute %v", listing.Dirs)
	}

	// Stato di yt-dlp dopo l'inizializzazione lato Kotlin.
	YtDlpInitialized()
	out, _ = Call("GetState", "")
	if !strings.Contains(out, `"ytDlpAvailable":true`) || !strings.Contains(out, `"ytDlpVersion":"2026.01.01"`) {
		t.Fatalf("stato yt-dlp inatteso: %s", out)
	}

	// Errori: metodo sconosciuto e numero di argomenti sbagliato.
	if _, err := Call("NonEsiste", "[]"); err == nil {
		t.Fatal("atteso errore per metodo sconosciuto")
	}
	if _, err := Call("SetFolder", "[]"); err == nil {
		t.Fatal("atteso errore per argomenti mancanti")
	}
}
