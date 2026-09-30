package core

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"renamemusic/internal/playlist"
	"renamemusic/internal/rules"
)

// fakeRunner simula yt-dlp: l'enumerazione restituisce i video di `titles`
// (id -> titolo) e il download di un video crea il relativo mp3 nella cartella
// indicata da -P home: col nome del modello -o, come farebbe yt-dlp.
type fakeRunner struct {
	titles map[string]string

	listArgs []string // argomenti dell'ultima enumerazione

	mu        sync.Mutex
	downloads int
}

func (r *fakeRunner) Run(args []string) ([]byte, []byte, error) {
	if args[0] == "--flat-playlist" {
		r.listArgs = args
		var out strings.Builder
		for id, title := range r.titles {
			out.WriteString(id + "\t" + title + "\n")
		}
		return []byte(out.String()), nil, nil
	}
	var home, tmpl string
	for i, a := range args {
		if a == "-o" {
			tmpl = args[i+1]
		}
		if v, ok := strings.CutPrefix(a, "home:"); ok && i > 0 && args[i-1] == "-P" {
			home = v
		}
	}
	url := args[len(args)-1]
	id := url[strings.LastIndex(url, "=")+1:]
	r.mu.Lock()
	r.downloads++
	r.mu.Unlock()
	path := filepath.Join(home, strings.NewReplacer("%(title)s", r.titles[id], "%(ext)s", "mp3").Replace(tmpl))
	return nil, nil, os.WriteFile(path, []byte("x"), 0o644)
}

// fakeYtDlp è una gestione di yt-dlp sempre disponibile che usa fakeRunner.
type fakeYtDlp struct{ runner *fakeRunner }

func (fakeYtDlp) ManagedPath() (string, error)            { return "yt-dlp", nil }
func (fakeYtDlp) Available(string) bool                   { return true }
func (fakeYtDlp) Version(string) string                   { return "test" }
func (fakeYtDlp) Install(string, playlist.Progress) error { return nil }
func (fakeYtDlp) Update(string) (bool, error)             { return false, nil }
func (fakeYtDlp) Uninstall(string) error                  { return nil }
func (fakeYtDlp) FFmpegAvailable() bool                   { return true }
func (fakeYtDlp) InstallFFmpeg(playlist.Progress) error   { return nil }
func (fakeYtDlp) FFmpegManaged() bool                     { return false }
func (fakeYtDlp) UninstallFFmpeg() error                  { return nil }
func (f fakeYtDlp) Runner(string) playlist.Runner         { return f.runner }
func (fakeYtDlp) Workers() int                            { return 1 }

// progressHost registra le fasi degli eventi di avanzamento.
type progressHost struct {
	nopHost
	mu     sync.Mutex
	phases []string
}

func (h *progressHost) Emit(event string, payload any) {
	if p, ok := payload.(ProgressEvent); ok && event == EventProcessProgress {
		h.mu.Lock()
		h.phases = append(h.phases, p.Phase)
		h.mu.Unlock()
	}
}

func newSimpleApp(t *testing.T, runner *fakeRunner, host Host) (*App, string) {
	t.Helper()
	cfgDir := t.TempDir()
	t.Setenv("AppData", cfgDir)
	t.Setenv("XDG_CONFIG_HOME", cfgDir)
	t.Setenv("HOME", cfgDir)

	musicDir := t.TempDir()
	cfg := rules.FactoryConfig()
	cfg.StartFolder = musicDir
	cfg.SimpleMode = true
	return &App{
		host:             host,
		ytdlp:            fakeYtDlp{runner: runner},
		config:           cfg,
		defaults:         cfg,
		playlists:        []playlist.Playlist{{Name: "Preferiti", URL: "https://example.com/playlist"}},
		destSameAsSource: true,
		ytDlpManaged:     true,
	}, musicDir
}

// TestDownloadAndProcess verifica il flusso della modalità semplificata: la
// playlist viene scaricata e i brani convertiti subito (nome normalizzato),
// con l'avanzamento prima del download e poi della conversione.
func TestDownloadAndProcess(t *testing.T) {
	runner := &fakeRunner{titles: map[string]string{"v1": "Artista - Titolo (Official Video)"}}
	host := &progressHost{}
	app, musicDir := newSimpleApp(t, runner, host)

	resp := app.DownloadAndProcess("Preferiti")
	if !resp.OK {
		t.Fatalf("DownloadAndProcess non ok: %s", resp.Message)
	}
	if len(resp.Results) != 1 || resp.Results[0].Failed {
		t.Fatalf("atteso 1 file convertito, ottenuti %+v", resp.Results)
	}
	if got := resp.Results[0].NewName; got != "Artista - Titolo.mp3" {
		t.Fatalf("nuovo nome atteso %q, ottenuto %q", "Artista - Titolo.mp3", got)
	}
	if _, err := os.Stat(filepath.Join(musicDir, "Artista - Titolo.mp3")); err != nil {
		t.Fatalf("file convertito assente: %v", err)
	}

	host.mu.Lock()
	phases := append([]string(nil), host.phases...)
	host.mu.Unlock()
	if len(phases) == 0 || phases[0] != PhaseDownload || phases[len(phases)-1] != PhaseConvert {
		t.Fatalf("fasi di avanzamento inattese: %v", phases)
	}
}

// TestDownloadAndProcessChecksDestinationFirst verifica che una destinazione
// non valida blocchi l'operazione PRIMA del download.
func TestDownloadAndProcessChecksDestinationFirst(t *testing.T) {
	runner := &fakeRunner{titles: map[string]string{"v1": "Artista - Titolo"}}
	app, _ := newSimpleApp(t, runner, nil)
	app.destSameAsSource = false
	app.destFolder = filepath.Join(t.TempDir(), "non-esiste")

	resp := app.DownloadAndProcess("Preferiti")
	if resp.OK {
		t.Fatal("atteso errore per destinazione inesistente")
	}
	if runner.downloads != 0 {
		t.Fatalf("nessun download atteso, eseguiti %d", runner.downloads)
	}
}

// TestDownloadLinkAndProcess verifica il download da un link incollato: si
// scarica solo il video del link (--no-playlist) e lo si converte subito.
func TestDownloadLinkAndProcess(t *testing.T) {
	runner := &fakeRunner{titles: map[string]string{"v1": "Artista - Titolo (Official Video)"}}
	app, musicDir := newSimpleApp(t, runner, nil)

	resp := app.DownloadLinkAndProcess("  https://www.youtube.com/watch?v=v1&list=PL1  ")
	if !resp.OK {
		t.Fatalf("DownloadLinkAndProcess non ok: %s", resp.Message)
	}
	if !slices.Contains(runner.listArgs, "--no-playlist") {
		t.Fatalf("atteso --no-playlist nell'enumerazione, argomenti %v", runner.listArgs)
	}
	if got := runner.listArgs[len(runner.listArgs)-1]; got != "https://www.youtube.com/watch?v=v1&list=PL1" {
		t.Fatalf("link passato a yt-dlp inatteso: %q", got)
	}
	if _, err := os.Stat(filepath.Join(musicDir, "Artista - Titolo.mp3")); err != nil {
		t.Fatalf("file convertito assente: %v", err)
	}
}

// TestDownloadLinkRejectsInvalid verifica che un link vuoto o non http(s)
// (es. un'opzione di yt-dlp) venga rifiutato senza avviare yt-dlp.
func TestDownloadLinkRejectsInvalid(t *testing.T) {
	runner := &fakeRunner{titles: map[string]string{"v1": "Artista - Titolo"}}
	app, _ := newSimpleApp(t, runner, nil)

	for _, link := range []string{"", "   ", "--exec calc", "youtube.com/watch?v=v1", "file:///etc/passwd"} {
		if resp := app.DownloadLink(link); resp.OK {
			t.Fatalf("link %q accettato", link)
		}
	}
	if runner.listArgs != nil || runner.downloads != 0 {
		t.Fatalf("yt-dlp non doveva partire: enumerazione %v, download %d", runner.listArgs, runner.downloads)
	}
}
