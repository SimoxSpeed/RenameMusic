package settings

import (
	"os"
	"path/filepath"
	"testing"
)

// cleanTempFilesIn deve rimuovere solo i temporanei col suffisso marcato dalla
// cartella di configurazione, senza toccare i file di config né .tmp estranei.
func TestCleanTempFilesIn(t *testing.T) {
	dir := t.TempDir()
	orphan := filepath.Join(dir, "config.json"+tempSuffix)
	foreign := filepath.Join(dir, "altro.tmp")
	cfg := filepath.Join(dir, "config.json")
	for _, p := range []string{orphan, foreign, cfg} {
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if removed := cleanTempFilesIn(dir); removed != 1 {
		t.Fatalf("rimossi %d file, attesi 1", removed)
	}
	if _, err := os.Stat(orphan); err == nil {
		t.Fatal("il temporaneo marcato doveva essere rimosso")
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatal("un .tmp estraneo NON deve essere rimosso")
	}
	if _, err := os.Stat(cfg); err != nil {
		t.Fatal("config.json NON deve essere rimosso")
	}
}
