package tags

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Round-trip: ciò che WriteMP3Tags scrive deve essere rileggibile identico da un
// parser indipendente (ReadMP3Tags), anche con caratteri non-ASCII. Verifica che
// l'encoder (dimensioni, BOM, UTF-16) sia corretto, non solo il layout dei byte.
func TestWriteReadRoundTrip(t *testing.T) {
	const audio = "questi-sono-byte-audio-fittizi"
	cases := []struct {
		name   string
		title  string
		artist string
	}{
		{"ascii", "Simple Title", "Simple Artist"},
		{"accenti", "Tïtolo con àccènti — VIP", "Àrtîst, Öther & Third"},
		{"non-latino", "日本語タイトル", "Ãrtïsta"},
		{"vuoti", "", ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "track.mp3")
			if err := os.WriteFile(path, []byte(audio), 0o644); err != nil {
				t.Fatal(err)
			}

			if err := WriteMP3Tags(path, c.title, c.artist); err != nil {
				t.Fatalf("WriteMP3Tags: %v", err)
			}

			gotTitle, gotArtist, err := ReadMP3Tags(path)
			if err != nil {
				t.Fatalf("ReadMP3Tags: %v", err)
			}
			if gotTitle != c.title {
				t.Fatalf("titolo: letto %q, atteso %q", gotTitle, c.title)
			}
			if gotArtist != c.artist {
				t.Fatalf("artista: letto %q, atteso %q", gotArtist, c.artist)
			}

			// L'audio originale deve restare in coda al file dopo la scrittura del tag.
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasSuffix(string(data), audio) {
				t.Fatal("il payload audio non è stato preservato")
			}
		})
	}
}

// Riscritture successive non devono accumulare tag: l'audio resta unico e i tag
// riletti sono sempre gli ultimi scritti (stripExistingTags rimuove i precedenti).
func TestWriteTwiceReplacesTag(t *testing.T) {
	const audio = "payload-audio"
	path := filepath.Join(t.TempDir(), "track.mp3")
	if err := os.WriteFile(path, []byte(audio), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := WriteMP3Tags(path, "Primo", "Uno"); err != nil {
		t.Fatal(err)
	}
	if err := WriteMP3Tags(path, "Secondo", "Due"); err != nil {
		t.Fatal(err)
	}

	title, artist, err := ReadMP3Tags(path)
	if err != nil {
		t.Fatal(err)
	}
	if title != "Secondo" || artist != "Due" {
		t.Fatalf("atteso Secondo/Due, letto %q/%q", title, artist)
	}
	data, _ := os.ReadFile(path)
	if strings.Count(string(data), audio) != 1 {
		t.Fatalf("l'audio dovrebbe comparire una sola volta, trovato %d volte", strings.Count(string(data), audio))
	}
}
