package rename

import (
	"os"
	"path/filepath"
	"testing"

	"renamemusic/internal/rules"
	"renamemusic/internal/tags"
)

// WriteTags scrive i tag e ne verifica la rilettura (round-trip). Sul percorso
// valido non deve dare errore e i tag riletti devono corrispondere al nome file.
func TestWriteTagsRoundTripVerifies(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Artist - Title.mp3")
	if err := os.WriteFile(path, []byte("audio-fittizio"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(rules.FactoryConfig())
	if err := svc.WriteTags(path); err != nil {
		t.Fatalf("WriteTags: %v", err)
	}

	title, artist, err := tags.ReadMP3Tags(path)
	if err != nil {
		t.Fatalf("ReadMP3Tags: %v", err)
	}
	if title != "Title" || artist != "Artist" {
		t.Fatalf("tag riletti = %q/%q, attesi Title/Artist", title, artist)
	}
}

// Sui file non-MP3 WriteTags non fa nulla (nessuna verifica, nessun errore).
func TestWriteTagsSkipsNonMP3(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Artist - Title.flac")
	if err := os.WriteFile(path, []byte("flac"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := NewService(rules.FactoryConfig()).WriteTags(path); err != nil {
		t.Fatalf("WriteTags su non-MP3 non deve fallire: %v", err)
	}
}
