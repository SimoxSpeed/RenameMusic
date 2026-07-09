package rename

import (
	"os"
	"path/filepath"
	"testing"

	"renamemusic/internal/rules"
)

// Process deve fermarsi quando Cancelled ritorna true, lasciando nei Result solo
// i file già elaborati prima dell'annullamento.
func TestProcessCancellation(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"A - X.mp3", "B - Y.mp3", "C - Z.mp3"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := rules.FactoryConfig()
	cfg.StartFolder = dir

	svc := NewService(cfg)
	files, err := svc.Scan()
	if err != nil {
		t.Fatal(err)
	}

	// Cancelled è interrogata prima di ogni file: consente i primi 2, annulla al 3°.
	checks := 0
	results, err := svc.Process(files, Options{
		DeleteOriginals: true,
		Cancelled:       func() bool { checks++; return checks > 2 },
	})
	if err != nil {
		t.Fatal(err)
	}
	// Tutti e 3 i file compaiono: 2 elaborati + 1 marcato Canceled (non elaborato).
	if len(results) != 3 {
		t.Fatalf("attesi 3 risultati, ottenuti %d", len(results))
	}
	processed, canceled := 0, 0
	for _, r := range results {
		if r.Canceled {
			canceled++
		} else {
			processed++
		}
	}
	if processed != 2 || canceled != 1 {
		t.Fatalf("attesi 2 elaborati + 1 annullato, ottenuti %d + %d", processed, canceled)
	}
}

// ClearTags deve fermarsi all'annullamento, ripulendo solo i file già processati.
func TestClearTagsCancellation(t *testing.T) {
	dir := t.TempDir()
	paths := make([]string, 0, 3)
	for _, n := range []string{"a.mp3", "b.mp3", "c.mp3"} {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}

	svc := NewService(rules.FactoryConfig())
	checks := 0
	cleared, failed := svc.ClearTags(paths, nil, func() bool { checks++; return checks > 1 })
	if cleared != 1 || failed != 0 {
		t.Fatalf("atteso 1 ripulito prima dell'annullamento, ottenuti cleared=%d failed=%d", cleared, failed)
	}
}
