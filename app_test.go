package main

import (
	"os"
	"path/filepath"
	"testing"

	"renamemusic/internal/rules"
)

func writeTmpFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestSetConfigRescansWithNewRules verifica che salvare le impostazioni riscansioni
// la cartella con le nuove regole: aggiungendo un'estensione supportata, un file
// prima escluso deve comparire nell'anteprima SENZA una scansione manuale. Copre
// anche la nuova normalizzazione (l'occorrenza "(Official Video)" rimossa dal nome).
func TestSetConfigRescansWithNewRules(t *testing.T) {
	// Isola la persistenza delle impostazioni in una cartella temporanea, così il
	// test non tocca la configurazione reale dell'utente (os.UserConfigDir).
	cfgDir := t.TempDir()
	t.Setenv("AppData", cfgDir)         // Windows
	t.Setenv("XDG_CONFIG_HOME", cfgDir) // Linux
	t.Setenv("HOME", cfgDir)            // macOS

	musicDir := t.TempDir()
	writeTmpFile(t, filepath.Join(musicDir, "Alpha (Official Video).mp3"))
	writeTmpFile(t, filepath.Join(musicDir, "Beta.wav"))

	// Config iniziale: supporta SOLO mp3.
	initial := rules.FactoryConfig()
	initial.StartFolder = musicDir
	initial.SupportedExtensions = []string{"mp3"}

	app := &App{config: initial, defaults: initial}

	// Scan iniziale: deve vedere solo il file mp3 (il wav è escluso).
	resp := app.Scan()
	if len(resp.State.Files) != 1 {
		t.Fatalf("scan iniziale: attesi 1 file (solo mp3), ottenuti %d", len(resp.State.Files))
	}

	// Salva nuove regole che AGGIUNGONO il supporto a wav.
	updated := initial
	updated.SupportedExtensions = []string{"mp3", "wav"}
	resp = app.SetConfig(updated)

	if !resp.OK {
		t.Fatalf("SetConfig non ok: %s", resp.Message)
	}
	// Senza il rescan al salvataggio qui resterebbe 1 (il wav non comparirebbe).
	if len(resp.State.Files) != 2 {
		t.Fatalf("dopo il salvataggio attesi 2 file (mp3+wav) dalla riscansione, ottenuti %d", len(resp.State.Files))
	}

	// L'anteprima deve applicare la normalizzazione: "(Official Video)" rimosso.
	var mp3Preview string
	for _, f := range resp.State.Files {
		if f.Name == "Alpha (Official Video).mp3" {
			mp3Preview = f.Preview
		}
	}
	if mp3Preview != "Alpha.mp3" {
		t.Fatalf("anteprima mp3 attesa \"Alpha.mp3\", ottenuta %q", mp3Preview)
	}
}

// TestSetConfigRescanReflectsRemovedExtension verifica il caso opposto: togliere
// un'estensione dal set supportato deve escludere quei file dall'anteprima dopo
// il salvataggio, senza scansione manuale.
func TestSetConfigRescanReflectsRemovedExtension(t *testing.T) {
	cfgDir := t.TempDir()
	t.Setenv("AppData", cfgDir)
	t.Setenv("XDG_CONFIG_HOME", cfgDir)
	t.Setenv("HOME", cfgDir)

	musicDir := t.TempDir()
	writeTmpFile(t, filepath.Join(musicDir, "Alpha.mp3"))
	writeTmpFile(t, filepath.Join(musicDir, "Beta.wav"))

	initial := rules.FactoryConfig()
	initial.StartFolder = musicDir
	initial.SupportedExtensions = []string{"mp3", "wav"}

	app := &App{config: initial, defaults: initial}

	if resp := app.Scan(); len(resp.State.Files) != 2 {
		t.Fatalf("scan iniziale: attesi 2 file, ottenuti %d", len(resp.State.Files))
	}

	updated := initial
	updated.SupportedExtensions = []string{"mp3"}
	resp := app.SetConfig(updated)

	if !resp.OK {
		t.Fatalf("SetConfig non ok: %s", resp.Message)
	}
	if len(resp.State.Files) != 1 {
		t.Fatalf("dopo aver rimosso wav attesi 1 file, ottenuti %d", len(resp.State.Files))
	}
	if resp.State.Files[0].Name != "Alpha.mp3" {
		t.Fatalf("file residuo atteso \"Alpha.mp3\", ottenuto %q", resp.State.Files[0].Name)
	}
}
