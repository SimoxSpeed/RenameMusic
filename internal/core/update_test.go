package core

import (
	"testing"

	"renamemusic/internal/settings"
	"renamemusic/internal/update"
)

// TestMarkUpdateSeenPersists verifica che il popup risulti "già mostrato" solo
// per la versione segnata, anche dopo un riavvio (state.json), e che una
// versione successiva torni a non esserlo.
func TestMarkUpdateSeenPersists(t *testing.T) {
	settings.SetDir(t.TempDir())
	t.Cleanup(func() { settings.SetDir("") })

	a := New(Options{})
	a.latestRelease = &update.Release{Version: "9.0.0"}
	if v := a.snapshotLocked().Update; v == nil || v.Seen {
		t.Fatalf("atteso aggiornamento non ancora visto, ottenuto %+v", v)
	}

	a.MarkUpdateSeen("9.0.0")
	if v := a.snapshotLocked().Update; v == nil || !v.Seen {
		t.Fatalf("atteso aggiornamento visto, ottenuto %+v", v)
	}

	restarted := New(Options{})
	restarted.latestRelease = &update.Release{Version: "9.0.0"}
	if v := restarted.snapshotLocked().Update; v == nil || !v.Seen {
		t.Fatal("dopo il riavvio il popup non deve ricomparire")
	}
	restarted.latestRelease = &update.Release{Version: "9.1.0"}
	if v := restarted.snapshotLocked().Update; v == nil || v.Seen {
		t.Fatal("una nuova versione deve mostrare di nuovo il popup")
	}
}
