package watcher

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"renamemusic/internal/rules"
)

// startTestWatcher avvia un watcher con un quiet period breve e restituisce
// una funzione che blocca finché l'handler non è stato invocato almeno una
// volta (con timeout), insieme allo slice dei basename ricevuti.
func startTestWatcher(t *testing.T, dir string, quiet time.Duration) (*Watcher, func(t *testing.T) []string) {
	t.Helper()

	w := New()
	w.QuietPeriod = quiet

	var (
		mu       sync.Mutex
		received []string
		fired    = make(chan struct{}, 8)
	)
	err := w.Start(dir, rules.FactoryConfig(), func(path string) {
		mu.Lock()
		received = append(received, filepath.Base(path))
		mu.Unlock()
		select {
		case fired <- struct{}{}:
		default:
		}
	}, nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	wait := func(t *testing.T) []string {
		t.Helper()
		select {
		case <-fired:
		case <-time.After(3 * time.Second):
			t.Fatal("handler non invocato entro il timeout")
		}
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), received...)
	}
	return w, wait
}

func TestWatcherDetectsNewSupportedFile(t *testing.T) {
	dir := t.TempDir()
	w, wait := startTestWatcher(t, dir, 50*time.Millisecond)
	defer w.Stop()

	target := filepath.Join(dir, "Nuovo Brano.mp3")
	if err := os.WriteFile(target, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := wait(t)
	if len(got) != 1 || got[0] != "Nuovo Brano.mp3" {
		t.Fatalf("handler ricevuto per %v, atteso [Nuovo Brano.mp3]", got)
	}
}

func TestWatcherIgnoresUnsupportedExtensions(t *testing.T) {
	dir := t.TempDir()

	w := New()
	w.QuietPeriod = 50 * time.Millisecond
	defer w.Stop()

	var (
		mu     sync.Mutex
		called int
	)
	err := w.Start(dir, rules.FactoryConfig(), func(string) {
		mu.Lock()
		called++
		mu.Unlock()
	}, nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Aspetta oltre il quiet period per lasciare al watcher il tempo di ignorare il file.
	time.Sleep(400 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if called != 0 {
		t.Fatalf("handler invocato %d volte per un file non supportato", called)
	}
}

func TestWatcherDetectsModificationOfExistingFile(t *testing.T) {
	dir := t.TempDir()

	// Il file esiste GIÀ prima di avviare il watcher: solo l'evento di modifica
	// dovrà triggerare l'handler.
	target := filepath.Join(dir, "Song.mp3")
	if err := os.WriteFile(target, []byte("initial"), 0o644); err != nil {
		t.Fatal(err)
	}

	w, wait := startTestWatcher(t, dir, 50*time.Millisecond)
	defer w.Stop()

	// Piccola pausa per assicurarsi che il watcher sia pronto.
	time.Sleep(100 * time.Millisecond)

	if err := os.WriteFile(target, []byte("modified"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := wait(t)
	if len(got) == 0 {
		t.Fatal("handler mai invocato per la modifica")
	}
}

func TestWatcherDetectsRemovalOfExistingFile(t *testing.T) {
	dir := t.TempDir()

	target := filepath.Join(dir, "Old.mp3")
	if err := os.WriteFile(target, []byte("bye"), 0o644); err != nil {
		t.Fatal(err)
	}

	w, wait := startTestWatcher(t, dir, 50*time.Millisecond)
	defer w.Stop()

	// Piccola pausa per assicurarsi che il watcher sia pronto.
	time.Sleep(100 * time.Millisecond)

	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}

	got := wait(t)
	if len(got) == 0 {
		t.Fatal("handler mai invocato per la rimozione")
	}
}

func TestWatcherStopSilencesPendingEvents(t *testing.T) {
	dir := t.TempDir()

	w := New()
	w.QuietPeriod = 300 * time.Millisecond

	var (
		mu     sync.Mutex
		called int
	)
	err := w.Start(dir, rules.FactoryConfig(), func(string) {
		mu.Lock()
		called++
		mu.Unlock()
	}, nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "song.mp3"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Ferma il watcher PRIMA che il timer di quiete scada.
	time.Sleep(50 * time.Millisecond)
	w.Stop()

	// Aspetta abbondantemente oltre il quiet period: nessuna invocazione attesa.
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if called != 0 {
		t.Fatalf("handler invocato %d volte dopo Stop", called)
	}
}
