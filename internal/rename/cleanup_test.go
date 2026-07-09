package rename

import (
	"os"
	"path/filepath"
	"testing"
)

// CleanTempFiles deve rimuovere SOLO i temporanei col suffisso marcato, lasciando
// intatti i file normali e gli eventuali ".tmp" estranei dell'utente.
func TestCleanTempFiles(t *testing.T) {
	dir := t.TempDir()
	orphan := filepath.Join(dir, "Song.mp3"+tempSuffix)
	foreign := filepath.Join(dir, "appunti-utente.tmp")
	normal := filepath.Join(dir, "Track.mp3")
	for _, p := range []string{orphan, foreign, normal} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if removed := CleanTempFiles(dir); removed != 1 {
		t.Fatalf("rimossi %d file, attesi 1", removed)
	}
	if _, err := os.Stat(orphan); err == nil {
		t.Fatal("il temporaneo marcato doveva essere rimosso")
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatal("un .tmp estraneo NON deve essere rimosso")
	}
	if _, err := os.Stat(normal); err != nil {
		t.Fatal("un file normale NON deve essere rimosso")
	}
}

func TestCleanTempFilesEmptyFolder(t *testing.T) {
	if removed := CleanTempFiles(""); removed != 0 {
		t.Fatalf("cartella vuota: rimossi %d, attesi 0", removed)
	}
}
