package playlist

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeRunner simula yt-dlp: all'enumerazione restituisce i video indicati, al
// download scrive un .part nella cartella temporanea (-P temp:) e, se il video
// non è tra quelli che falliscono, l'mp3 finito nella cartella home (-P home:).
type fakeRunner struct {
	videos []string
	fail   map[string]bool
}

func (f fakeRunner) Run(args []string) ([]byte, []byte, error) {
	if args[0] == "--flat-playlist" {
		var out strings.Builder
		for _, id := range f.videos {
			out.WriteString(id + "\tTitolo " + id + "\n")
		}
		return []byte(out.String()), nil, nil
	}
	var home, temp, id string
	for i, a := range args {
		if a == "-P" && i+1 < len(args) {
			if v, ok := strings.CutPrefix(args[i+1], "home:"); ok {
				home = v
			}
			if v, ok := strings.CutPrefix(args[i+1], "temp:"); ok {
				temp = v
			}
		}
		// Con un -o assoluto yt-dlp ignorerebbe la cartella temporanea.
		if a == "-o" && i+1 < len(args) && filepath.IsAbs(args[i+1]) {
			return nil, []byte("ERROR: modello -o assoluto"), errors.New("exit 1")
		}
	}
	url := args[len(args)-1]
	id = url[strings.LastIndex(url, "=")+1:]
	if home == "" || temp == "" {
		return nil, []byte("ERROR: cartelle mancanti"), errors.New("exit 1")
	}
	if err := os.MkdirAll(temp, 0o755); err != nil {
		return nil, nil, err
	}
	if err := os.WriteFile(filepath.Join(temp, id+".webm.part"), []byte("parziale"), 0o644); err != nil {
		return nil, nil, err
	}
	if f.fail[id] {
		return nil, []byte("ERROR: rete interrotta"), errors.New("exit 1")
	}
	return nil, nil, os.WriteFile(filepath.Join(home, id+".mp3"), []byte("mp3"), 0o644)
}

// I file intermedi di yt-dlp non devono restare nella cartella: né quelli di
// un download precedente interrotto, né quelli dei download (riusciti o
// falliti) appena fatti.
func TestDownloadLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, TempDirName, "vecchio", "brano.webm.part")
	if err := os.MkdirAll(filepath.Dir(stale), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("residuo"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Download(Options{
		Runner: fakeRunner{videos: []string{"ok1", "ko", "ok2"}, fail: map[string]bool{"ko": true}},
		URL:    "https://www.youtube.com/playlist?list=x",
		Folder: dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Downloaded != 2 || res.Failed != 1 {
		t.Fatalf("scaricati %d, falliti %d; attesi 2 e 1", res.Downloaded, res.Failed)
	}
	if _, err := os.Stat(filepath.Join(dir, TempDirName)); !os.IsNotExist(err) {
		t.Fatalf("la cartella dei file intermedi deve sparire, stat: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if strings.Join(names, ",") != "ok1.mp3,ok2.mp3" {
		t.Fatalf("nella cartella solo gli mp3 finiti, trovati: %v", names)
	}
}

func TestCleanTempDirEmptyFolder(t *testing.T) {
	if err := CleanTempDir(""); err != nil {
		t.Fatalf("cartella vuota: nessun errore, ottenuto %v", err)
	}
}
