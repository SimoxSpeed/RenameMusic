package core

import (
	"os"
	"path/filepath"
	"testing"
)

// TestProcessAllReview verifica che le tracce selezionate da rivedere non
// vengano convertite subito ma tornino alla UI come tracce da confermare, con
// il nome proposto dall'anteprima, mentre le altre si convertono normalmente.
func TestProcessAllReview(t *testing.T) {
	app, musicDir := newSimpleApp(t, &fakeRunner{}, nil)
	app.config.SimpleMode = false
	one := filepath.Join(musicDir, "Artista - Uno (Official Video).mp3")
	two := filepath.Join(musicDir, "Artista - Due (Official Video).mp3")
	for _, p := range []string{one, two} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if resp := app.Scan(); !resp.OK {
		t.Fatalf("Scan non ok: %s", resp.Message)
	}

	resp := app.ProcessAll([]string{two})
	if len(resp.Results) != 1 || resp.Results[0].NewName != "Artista - Uno.mp3" {
		t.Fatalf("atteso solo %q convertito, ottenuti %+v", "Artista - Uno.mp3", resp.Results)
	}
	if len(resp.Prompts) != 1 {
		t.Fatalf("attesa 1 traccia da rivedere, ottenute %+v", resp.Prompts)
	}
	p := resp.Prompts[0]
	if p.Path != two || !p.Review || p.PreviewBase != "Artista - Due" {
		t.Fatalf("traccia da rivedere inattesa: %+v", p)
	}
	if _, err := os.Stat(two); err != nil {
		t.Fatalf("la traccia da rivedere non doveva essere toccata: %v", err)
	}
}
