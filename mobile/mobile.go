// Package mobile è il punto d'ingresso dell'app Android. Viene compilato con
// `gomobile bind` in una libreria .aar (classe Java mobile.Mobile) chiamata dal
// plugin Capacitor in Kotlin (frontend/android).
//
// Espone lo stesso core dell'app desktop (internal/core) con un'API minima e a
// tipi semplici, come richiesto da gomobile: la UI invoca i metodi di core.App
// per nome con argomenti e risultato in JSON (Call), esattamente come Wails fa
// su desktop con i binding generati. Ciò che dipende dalla piattaforma (eventi
// verso la UI, yt-dlp tramite youtubedl-android) passa dall'interfaccia Host,
// implementata in Kotlin.
package mobile

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"renamemusic/internal/core"
	"renamemusic/internal/settings"
)

// Host è implementata dal lato Kotlin dell'app.
type Host interface {
	// Emit inoltra alla UI (WebView) un evento del core con payload JSON.
	Emit(event string, payloadJSON string)
	// YtDlpRun esegue yt-dlp (youtubedl-android) con gli argomenti indicati
	// (array JSON di stringhe) e restituisce un oggetto JSON
	// {"stdout": ..., "stderr": ..., "error": ...}; "error" vuoto = successo.
	// È bloccante e può essere chiamata da più goroutine in parallelo.
	YtDlpRun(argsJSON string) string
	// YtDlpReady indica se youtubedl-android è inizializzato e utilizzabile.
	YtDlpReady() bool
	// YtDlpUpdate aggiorna yt-dlp all'ultima versione stabile: restituisce ""
	// se va a buon fine, altrimenti il messaggio d'errore.
	YtDlpUpdate() string
}

var (
	mu      sync.Mutex
	app     *core.App
	ytdlp   *androidYtDlp
	folders = &folderAPI{}
)

// Start inizializza il core usando dataDir (la cartella privata dell'app,
// Context.getFilesDir) per la configurazione. Idempotente: le chiamate
// successive (es. Activity ricreata dopo una rotazione) non fanno nulla, perché
// lo stato vive per tutta la durata del processo.
func Start(dataDir string, host Host) error {
	mu.Lock()
	defer mu.Unlock()
	if app != nil {
		return nil
	}
	if dataDir == "" || host == nil {
		return errors.New("dataDir e host sono obbligatori")
	}
	settings.SetDir(dataDir)
	ytdlp = &androidYtDlp{host: host}
	app = core.New(core.Options{Host: mobileHost{host: host}, YtDlp: ytdlp})
	core.Start(app)
	return nil
}

// Call invoca per nome un metodo della UI (i metodi esportati di core.App, più
// quelli di folderAPI) con gli argomenti in un array JSON, e ne restituisce il
// risultato serializzato in JSON. È l'equivalente Android dei binding Wails.
// Può essere chiamata in parallelo da più thread: il core è già protetto da
// lock (come su desktop, dove Wails invoca i metodi in concorrenza).
func Call(method string, argsJSON string) (result string, err error) {
	mu.Lock()
	a := app
	mu.Unlock()
	if a == nil {
		return "", errors.New("core non inizializzato: chiamare Start")
	}

	// Un panic nel core farebbe crashare l'intera app Android: lo trasformiamo
	// in un errore, che la UI mostra come qualsiasi altra chiamata fallita.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("errore interno in %s: %v", method, r)
		}
	}()

	target := reflect.ValueOf(a).MethodByName(method)
	if !target.IsValid() {
		target = reflect.ValueOf(folders).MethodByName(method)
	}
	if !target.IsValid() {
		return "", fmt.Errorf("metodo sconosciuto: %s", method)
	}

	var raw []json.RawMessage
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &raw); err != nil {
			return "", fmt.Errorf("argomenti non validi per %s: %w", method, err)
		}
	}
	t := target.Type()
	if len(raw) != t.NumIn() {
		return "", fmt.Errorf("%s richiede %d argomenti, ricevuti %d", method, t.NumIn(), len(raw))
	}
	in := make([]reflect.Value, t.NumIn())
	for i := range in {
		p := reflect.New(t.In(i))
		if err := json.Unmarshal(raw[i], p.Interface()); err != nil {
			return "", fmt.Errorf("argomento %d non valido per %s: %w", i+1, method, err)
		}
		in[i] = p.Elem()
	}

	out := target.Call(in)
	var value any
	if len(out) > 0 {
		value = out[0].Interface()
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("risultato di %s non serializzabile: %w", method, err)
	}
	return string(data), nil
}

// Resume va chiamata quando l'app torna in primo piano: se l'aggiornamento
// automatico è attivo, riscansiona la cartella (può essere cambiata mentre
// l'app era in background, anche senza eventi dal watcher).
func Resume() {
	mu.Lock()
	a := app
	mu.Unlock()
	if a != nil {
		core.NotifyExternalChange(a)
	}
}

// YtDlpInitialized va chiamata dal lato Kotlin al termine (riuscito o meno)
// dell'inizializzazione di youtubedl-android, che avviene in background perché
// al primo avvio estrae Python e richiede qualche secondo. Rilegge disponibilità
// e versione e notifica la UI (evento core.EventYtDlpChanged).
func YtDlpInitialized() {
	mu.Lock()
	a, y := app, ytdlp
	mu.Unlock()
	if a == nil || y == nil {
		return
	}
	y.load()
	core.RefreshYtDlp(a)
}

// mobileHost adatta Host (Kotlin, payload in JSON) a core.Host. I selettori di
// sistema non servono: su Android la UI usa il proprio selettore di cartelle
// (folderAPI) e yt-dlp è integrato, quindi non si sceglie alcun eseguibile.
type mobileHost struct {
	host Host
}

var errUnsupported = errors.New("funzione non disponibile su Android")

func (h mobileHost) Emit(event string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	h.host.Emit(event, string(data))
}

func (mobileHost) ChooseDirectory(string) (string, error)               { return "", errUnsupported }
func (mobileHost) ChooseFile(string, []core.FileFilter) (string, error) { return "", errUnsupported }
func (mobileHost) OpenFolder(string) error                              { return errUnsupported }
