package settings

import (
	"testing"

	"renamemusic/internal/rules"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	// os.UserConfigDir usa %AppData% su Windows: lo puntiamo a una dir temporanea.
	t.Setenv("AppData", t.TempDir())

	// Prima del salvataggio: nessun file -> seed di fabbrica, existed=false.
	cfg, existed, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig iniziale: %v", err)
	}
	if existed {
		t.Fatal("existed dovrebbe essere false quando il file non c'e'")
	}

	cfg.StartFolder = `C:\Musica\Test`
	cfg.Replacements = append(cfg.Replacements, rules.Replacement{From: "AAA", To: "bbb"})

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	loaded, existed, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig dopo Save: %v", err)
	}
	if !existed {
		t.Fatal("existed dovrebbe essere true dopo Save")
	}
	if loaded.StartFolder != `C:\Musica\Test` {
		t.Fatalf("StartFolder = %q, atteso C:\\Musica\\Test", loaded.StartFolder)
	}
	last := loaded.Replacements[len(loaded.Replacements)-1]
	if last.From != "AAA" || last.To != "bbb" {
		t.Fatalf("replacement persistito errato: %+v", last)
	}
}
