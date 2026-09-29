package core

import (
	"path/filepath"
	"strings"
	"testing"

	"renamemusic/internal/rules"
)

// TestScanRefusesMissingFolders verifica che la scansione venga rifiutata (con
// avviso e anteprima svuotata) se la cartella di partenza o quella di
// destinazione non esistono più, e che lo stato lo segnali alla UI.
func TestScanRefusesMissingFolders(t *testing.T) {
	cfgDir := t.TempDir()
	t.Setenv("AppData", cfgDir)
	t.Setenv("XDG_CONFIG_HOME", cfgDir)
	t.Setenv("HOME", cfgDir)

	musicDir := t.TempDir()
	writeTmpFile(t, filepath.Join(musicDir, "Alpha.mp3"))
	missing := filepath.Join(t.TempDir(), "sparita")

	cfg := rules.FactoryConfig()
	cfg.StartFolder = musicDir
	app := &App{config: cfg, defaults: cfg, destSameAsSource: true}

	if resp := app.Scan(); !resp.OK || len(resp.State.Files) != 1 {
		t.Fatalf("scan iniziale: ok=%v, %d file", resp.OK, len(resp.State.Files))
	}

	// Destinazione distinta che non esiste: scansione rifiutata.
	app.destSameAsSource = false
	app.destFolder = missing
	resp := app.Scan()
	if resp.OK || !strings.Contains(resp.Message, "destinazione non trovata") {
		t.Fatalf("destinazione mancante: ok=%v, messaggio %q", resp.OK, resp.Message)
	}
	if !resp.State.DestinationMissing || resp.State.FolderMissing || len(resp.State.Files) != 0 {
		t.Fatalf("destinazione mancante: stato inatteso %+v", resp.State)
	}

	// Destinazione non ancora scelta: la scansione è permessa.
	app.destFolder = ""
	if resp := app.Scan(); !resp.OK || resp.State.DestinationMissing {
		t.Fatalf("destinazione non scelta: ok=%v, %q", resp.OK, resp.Message)
	}

	// Cartella di partenza che non esiste: scansione e conversione rifiutate.
	app.destSameAsSource = true
	app.config.StartFolder = missing
	resp = app.Scan()
	if resp.OK || !strings.Contains(resp.Message, "partenza non trovata") {
		t.Fatalf("partenza mancante: ok=%v, messaggio %q", resp.OK, resp.Message)
	}
	if !resp.State.FolderMissing || len(resp.State.Files) != 0 {
		t.Fatalf("partenza mancante: stato inatteso %+v", resp.State)
	}
	if resp := app.ProcessAll(nil); resp.OK {
		t.Fatalf("conversione con partenza mancante accettata")
	}
}
