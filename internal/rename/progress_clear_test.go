package rename

import (
	"os"
	"path/filepath"
	"testing"

	"renamemusic/internal/rules"
	"renamemusic/internal/tags"
)

// ClearTags cancella i tag solo dagli MP3 e ne conta l'esito; ignora i non-MP3.
func TestServiceClearTags(t *testing.T) {
	dir := t.TempDir()
	mp3 := filepath.Join(dir, "Artist - Title.mp3")
	flac := filepath.Join(dir, "Something.flac")
	if err := os.WriteFile(mp3, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(flac, []byte("flac"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(rules.FactoryConfig())
	if err := svc.WriteTags(mp3); err != nil {
		t.Fatal(err)
	}

	cleared, failed := svc.ClearTags([]string{mp3, flac}, nil, nil)
	if cleared != 1 || failed != 0 {
		t.Fatalf("cleared=%d failed=%d, attesi 1/0", cleared, failed)
	}

	// Dopo la cancellazione l'MP3 non ha più un header ID3 leggibile.
	if _, _, err := tags.ReadMP3Tags(mp3); err == nil {
		t.Fatal("i tag dell'MP3 dovevano essere assenti dopo ClearTags")
	}
}

// OnProgress viene invocata una volta per file supportato, con done crescente e
// total costante pari al numero di file supportati.
func TestProcessOnProgress(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"A - X.mp3", "B - Y.mp3", "C - Z.flac"} {
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

	var calls [][2]int
	if _, err := svc.Process(files, Options{
		DeleteOriginals: true,
		OnProgress:      func(done, total int) { calls = append(calls, [2]int{done, total}) },
	}); err != nil {
		t.Fatal(err)
	}

	if len(calls) != 3 {
		t.Fatalf("attese 3 chiamate di progress, ottenute %d", len(calls))
	}
	for i, c := range calls {
		if c[0] != i+1 || c[1] != 3 {
			t.Fatalf("progress %d = (done=%d,total=%d), atteso (done=%d,total=3)", i, c[0], c[1], i+1)
		}
	}
}
