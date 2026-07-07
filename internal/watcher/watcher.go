// Package watcher osserva una cartella (non ricorsivamente) per la comparsa,
// modifica o rimozione di file audio supportati e invoca un Handler dopo un
// breve periodo di quiete, in modo da non elaborare file ancora in scrittura.
package watcher

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"renamemusic/internal/parser"
	"renamemusic/internal/rules"
)

// Handler viene invocato per ciascun file audio "stabile" nella cartella
// osservata (creazione, modifica o rimozione). Riceve il percorso assoluto.
type Handler func(path string)

// ErrorHandler viene invocato per gli errori non fatali del watcher (es. errori
// riportati da fsnotify sul canale Errors). Può essere nil.
type ErrorHandler func(err error)

// DefaultQuietPeriod è il tempo di quiete atteso dopo l'ultimo evento su un file
// prima di considerarlo "stabile" e passarlo all'Handler.
const DefaultQuietPeriod = 150 * time.Millisecond

// interestingOps è la maschera di eventi fsnotify che vogliamo osservare.
const interestingOps = fsnotify.Create | fsnotify.Write | fsnotify.Remove | fsnotify.Rename

// Watcher osserva una cartella e coalescia gli eventi per file.
type Watcher struct {
	// QuietPeriod è il tempo di quiete richiesto prima di invocare Handler.
	// Se zero viene usato DefaultQuietPeriod.
	QuietPeriod time.Duration

	// Debug abilita la stampa su stderr di ogni evento fsnotify ricevuto,
	// utile per diagnosticare pattern di scrittura di editor esterni.
	Debug bool

	mu      sync.Mutex
	inner   *fsnotify.Watcher
	cancel  context.CancelFunc
	folder  string
	pending map[string]*time.Timer
}

// New crea un Watcher inattivo. Chiama Start per iniziare l'osservazione.
func New() *Watcher {
	return &Watcher{QuietPeriod: DefaultQuietPeriod}
}

// Start avvia l'osservazione della cartella indicata. Se il Watcher è già attivo
// viene fermato prima. cfg viene usata solo per filtrare le estensioni
// supportate al momento dell'evento.
func (w *Watcher) Start(folder string, cfg rules.Config, onFile Handler, onError ErrorHandler) error {
	if folder == "" {
		return errors.New("watcher: cartella non specificata")
	}
	if onFile == nil {
		return errors.New("watcher: handler nullo")
	}

	w.Stop()

	inner, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err := inner.Add(folder); err != nil {
		_ = inner.Close()
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())

	w.mu.Lock()
	w.inner = inner
	w.cancel = cancel
	w.folder = folder
	w.pending = make(map[string]*time.Timer)
	quiet := w.QuietPeriod
	if quiet <= 0 {
		quiet = DefaultQuietPeriod
	}
	w.mu.Unlock()

	go w.loop(ctx, inner, cfg, quiet, onFile, onError)
	return nil
}

// Stop ferma l'osservazione (idempotente). Gli eventi in attesa nel periodo di
// quiete vengono scartati.
func (w *Watcher) Stop() {
	w.mu.Lock()
	cancel := w.cancel
	inner := w.inner
	for name, t := range w.pending {
		t.Stop()
		delete(w.pending, name)
	}
	w.inner = nil
	w.cancel = nil
	w.folder = ""
	w.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if inner != nil {
		_ = inner.Close()
	}
}

// Folder restituisce la cartella attualmente osservata ("" se il Watcher è fermo).
func (w *Watcher) Folder() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.folder
}

func (w *Watcher) loop(
	ctx context.Context,
	inner *fsnotify.Watcher,
	cfg rules.Config,
	quiet time.Duration,
	onFile Handler,
	onError ErrorHandler,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-inner.Events:
			if !ok {
				return
			}
			w.handleEvent(ev, cfg, quiet, onFile)
		case err, ok := <-inner.Errors:
			if !ok {
				return
			}
			if onError != nil && err != nil {
				onError(err)
			}
		}
	}
}

func (w *Watcher) handleEvent(ev fsnotify.Event, cfg rules.Config, quiet time.Duration, onFile Handler) {
	if w.Debug {
		fmt.Fprintf(os.Stderr, "[watcher] raw event op=%s name=%s\n", ev.Op.String(), ev.Name)
	}
	if ev.Op&interestingOps == 0 {
		return
	}
	if !cfg.IsSupportedExtension(parser.Extension(filepath.Base(ev.Name))) {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.inner == nil {
		return
	}
	if t, ok := w.pending[ev.Name]; ok {
		t.Reset(quiet)
		return
	}
	path := ev.Name
	w.pending[path] = time.AfterFunc(quiet, func() {
		w.mu.Lock()
		// Se Stop() è arrivato mentre il timer stava per scattare, l'inner è
		// nil: evitiamo di invocare l'handler per uno stato ormai smontato.
		stopped := w.inner == nil
		delete(w.pending, path)
		w.mu.Unlock()
		if stopped {
			return
		}
		onFile(path)
	})
}
