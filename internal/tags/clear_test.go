package tags

import (
	"os"
	"path/filepath"
	"testing"
)

// ClearMP3Tags deve rimuovere l'header ID3 lasciando intatto l'audio, ed essere
// idempotente su un file già ripulito.
func TestClearMP3Tags(t *testing.T) {
	const audio = "payload-audio-fittizio"
	path := filepath.Join(t.TempDir(), "track.mp3")
	if err := os.WriteFile(path, []byte(audio), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := WriteMP3Tags(path, "Title", "Artist"); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); string(data[:3]) != "ID3" {
		t.Fatal("atteso header ID3 dopo la scrittura")
	}

	if err := ClearMP3Tags(path); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if len(data) >= 3 && string(data[:3]) == "ID3" {
		t.Fatal("i tag non sono stati cancellati")
	}
	if string(data) != audio {
		t.Fatalf("audio non preservato: %q", string(data))
	}

	// Idempotente: ripulire di nuovo non cambia nulla.
	if err := ClearMP3Tags(path); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); string(data) != audio {
		t.Fatal("ClearMP3Tags non idempotente")
	}
}
