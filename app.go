package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	appfs "renamemusic/internal/fs"
	"renamemusic/internal/parser"
	"renamemusic/internal/rename"
	"renamemusic/internal/rules"
	"renamemusic/internal/settings"
	"renamemusic/internal/watcher"
)

// EventWatchChanged è l'evento Wails emesso quando la modalità watch rileva
// una variazione nella cartella osservata: il payload è lo StateResponse
// aggiornato (con la nuova lista di file/anteprime), così l'UI può aggiornare
// solo l'anteprima senza avviare nulla — la conversione resta sempre manuale.
const EventWatchChanged = "watch:changed"

type App struct {
	ctx      context.Context
	mu       sync.Mutex
	config   rules.Config
	defaults rules.Config
	scanned  []string
	logs     []LogEntry

	// Opzioni di elaborazione persistite (state.json).
	destSameAsSource bool
	destFolder       string
	deleteOriginals  bool
	watchEnabled     bool

	watcher *watcher.Watcher

	// watchPaused sospende le notifiche del watcher finché non arriva un nuovo
	// Scan (o cambio cartella). Viene alzato al termine di un ProcessAll: in UI
	// coincide con la fase in cui è visibile il tasto "Avvia nuova scansione",
	// ed è il modo più semplice per ignorare gli eventi fsnotify che noi stessi
	// abbiamo appena generato.
	watchPaused bool

	// watchDebounce coalisce eventi ravvicinati in un unico rescan globale.
	watchDebounce *time.Timer
}

type FileView struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Preview string `json:"preview"`
	MP3     bool   `json:"mp3"`
}

type ResultView struct {
	OldName string `json:"oldName"`
	NewName string `json:"newName"`
	Tagged  bool   `json:"tagged"`
	Skipped bool   `json:"skipped"`
	Reason  string `json:"reason"`
}

// LogKind classifica la natura di una riga di attività. Viene assegnata alla
// SORGENTE (quando il messaggio viene emesso), così il frontend non deve più
// dedurla dal testo: è un dato esplicito, non un'euristica fragile.
type LogKind string

const (
	LogInfo    LogKind = "info"
	LogSuccess LogKind = "success"
	LogError   LogKind = "error"
	LogAuto    LogKind = "auto" // eventi legati all'aggiornamento automatico
)

// LogEntry è una riga di attività strutturata: orario, categoria e messaggio
// separati, pronti per essere renderizzati senza parsing lato UI.
type LogEntry struct {
	Time    string  `json:"time"`
	Kind    LogKind `json:"kind"`
	Message string  `json:"message"`
}

type StateResponse struct {
	Folder                  string       `json:"folder"`
	Files                   []FileView   `json:"files"`
	Logs                    []LogEntry   `json:"logs"`
	Config                  rules.Config `json:"config"`
	DestinationSameAsSource bool         `json:"destinationSameAsSource"`
	DestinationFolder       string       `json:"destinationFolder"`
	DeleteOriginals         bool         `json:"deleteOriginals"`
	WatchEnabled            bool         `json:"watchEnabled"`
	WatchActive             bool         `json:"watchActive"`
}

type ActionResponse struct {
	OK      bool          `json:"ok"`
	Message string        `json:"message"`
	State   StateResponse `json:"state"`
	Results []ResultView  `json:"results,omitempty"`
}

func NewApp() *App {
	logs := []LogEntry{newLogEntry(LogInfo, "App pronta.")}

	// I default sono persistiti: se il file non esiste lo si crea dal seed di fabbrica.
	defaults, defExisted, err := settings.LoadDefaults()
	if err != nil {
		logs = []LogEntry{newLogEntry(LogError, "Impossibile leggere i default salvati: uso il seed di fabbrica.")}
	}
	if !defExisted {
		_ = settings.SaveDefaults(defaults)
	}

	// Le regole correnti: se il file non esiste si inizializzano dai default.
	current, curExisted, err := settings.LoadConfig()
	if err != nil {
		logs = []LogEntry{newLogEntry(LogError, "Impossibile leggere la configurazione salvata: uso i default.")}
	}
	if !curExisted {
		current = defaults
		_ = settings.SaveConfig(current)
	} else {
		logs = []LogEntry{newLogEntry(LogInfo, "Configurazione caricata dal file salvato.")}
	}

	// Cartella e opzioni sono persistite a parte: al primo avvio la cartella è vuota.
	st, _ := settings.LoadState()
	current.StartFolder = st.LastFolder
	defaults.StartFolder = st.LastFolder

	w := watcher.New()
	if os.Getenv("RENAMEMUSIC_WATCH_DEBUG") != "" {
		w.Debug = true
	}

	return &App{
		config:           current,
		defaults:         defaults,
		logs:             logs,
		destSameAsSource: st.DestinationSameAsSource,
		destFolder:       st.DestinationFolder,
		deleteOriginals:  st.DeleteOriginals,
		watchEnabled:     st.WatchEnabled,
		watcher:          w,
	}
}

// persistStateLocked salva su disco cartella + opzioni. Va chiamata con il lock acquisito.
func (a *App) persistStateLocked() {
	_ = settings.SaveState(settings.State{
		LastFolder:              a.config.StartFolder,
		DestinationSameAsSource: a.destSameAsSource,
		DestinationFolder:       a.destFolder,
		DeleteOriginals:         a.deleteOriginals,
		WatchEnabled:            a.watchEnabled,
	})
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Se il watch era abilitato nella sessione precedente e c'è una cartella
	// ricordata, riavvialo automaticamente.
	a.mu.Lock()
	shouldStart := a.watchEnabled && appfs.IsDir(a.config.StartFolder)
	a.mu.Unlock()
	if shouldStart {
		if err := a.startWatcher(); err != nil {
			a.mu.Lock()
			a.watchEnabled = false
			a.persistStateLocked()
			a.addLogLocked(LogError, "Impossibile avviare l'aggiornamento automatico: "+err.Error())
			a.mu.Unlock()
		}
	}
}

func (a *App) GetState() ActionResponse {
	// Al primo accesso, se c'è una cartella ricordata ma non è ancora stata
	// scansionata, la scansioniamo QUI: così la prima risposta contiene già le
	// anteprime e la UI si popola in un solo passaggio, senza mostrare prima uno
	// stato vuoto e poi riempirlo (niente "azzera e ripopola" al refresh).
	msg := a.ensureScanned()
	return ActionResponse{OK: true, Message: msg, State: a.snapshotLocked()}
}

// ensureScanned esegue una scansione una tantum se la cartella è valida e non è
// ancora stato prodotto alcun risultato (a.scanned == nil). Restituisce il
// messaggio di stato da mostrare (vuoto se non ha scansionato). È idempotente:
// una volta che a.scanned è valorizzato (anche a lista vuota) non riscansiona.
func (a *App) ensureScanned() string {
	a.mu.Lock()
	needScan := a.scanned == nil && appfs.IsDir(a.config.StartFolder)
	cfg := a.config
	a.mu.Unlock()
	if !needScan {
		return ""
	}

	files, err := rename.NewService(cfg).Scan()
	if err != nil {
		a.mu.Lock()
		a.addLogLocked(LogError, "Errore scansione iniziale: "+err.Error())
		a.mu.Unlock()
		return "Errore scansione: " + err.Error()
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.scanned != nil {
		return "" // una scansione concorrente ha già valorizzato lo stato
	}
	a.scanned = files
	a.watchPaused = false
	a.addLogLocked(LogInfo, fmt.Sprintf("Scansione completata: %d file audio.", len(files)))
	return "Scansione completata."
}

// ClearLogs svuota il registro delle attività.
func (a *App) ClearLogs() ActionResponse {
	a.mu.Lock()
	a.logs = nil
	state := a.snapshot()
	a.mu.Unlock()
	return ActionResponse{OK: true, Message: "Attività pulita.", State: state}
}

// GetConfig restituisce lo stato includendo la configurazione corrente.
func (a *App) GetConfig() ActionResponse {
	return ActionResponse{OK: true, State: a.snapshotLocked()}
}

// SetOptions aggiorna e persiste le opzioni di elaborazione (destinazione + eliminazione originali).
func (a *App) SetOptions(destSameAsSource bool, destination string, deleteOriginals bool) ActionResponse {
	destination = strings.Trim(destination, `" `)

	a.mu.Lock()
	a.destSameAsSource = destSameAsSource
	a.destFolder = destination
	a.deleteOriginals = deleteOriginals
	a.persistStateLocked()
	state := a.snapshot()
	a.mu.Unlock()

	return ActionResponse{OK: true, State: state}
}

func (a *App) SetFolder(path string) ActionResponse {
	path = strings.Trim(path, `" `)
	if !appfs.IsDir(path) {
		return ActionResponse{OK: false, Message: "La cartella indicata non esiste.", State: a.snapshotLocked()}
	}

	a.mu.Lock()
	a.config.StartFolder = path
	a.scanned = nil
	a.watchPaused = false
	a.persistStateLocked()
	a.addLogLocked(LogInfo, "Cartella selezionata: "+path)
	watchWanted := a.watchEnabled
	state := a.snapshot()
	a.mu.Unlock()

	if watchWanted {
		if err := a.startWatcher(); err != nil {
			a.mu.Lock()
			a.addLogLocked(LogError, "Aggiornamento automatico non riavviato sulla nuova cartella: "+err.Error())
			state = a.snapshot()
			a.mu.Unlock()
		}
	}

	return ActionResponse{OK: true, Message: "Cartella selezionata.", State: state}
}

func (a *App) SelectFolder() ActionResponse {
	path, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Seleziona cartella Rename Music",
	})
	if err != nil {
		return ActionResponse{OK: false, Message: "Impossibile aprire il selettore cartella.", State: a.snapshotLocked()}
	}
	if path == "" {
		return ActionResponse{OK: false, Message: "Selezione annullata.", State: a.snapshotLocked()}
	}
	return a.SetFolder(path)
}

// SetConfig applica una nuova configurazione di regole/cartella (correnti) e la salva su disco.
// Le voci vuote vengono scartate. Non tocca i default.
func (a *App) SetConfig(cfg rules.Config) ActionResponse {
	cfg = normalizeConfig(cfg)

	a.mu.Lock()
	cfg.StartFolder = a.config.StartFolder // la cartella si gestisce a parte, non la tocchiamo
	a.mu.Unlock()

	saveErr := settings.SaveConfig(cfg)

	a.mu.Lock()
	a.config = cfg
	// NON azzeriamo scanned: le anteprime restano e si ricalcolano con le nuove regole.
	if saveErr != nil {
		a.addLogLocked(LogError, "Configurazione applicata ma NON salvata: "+saveErr.Error())
	} else {
		a.addLogLocked(LogSuccess, "Configurazione salvata.")
	}
	watchWanted := a.watchEnabled
	state := a.snapshot()
	a.mu.Unlock()

	if watchWanted {
		_ = a.startWatcher() // eventuali errori restano visibili nel log al prossimo cambio.
	}

	if saveErr != nil {
		return ActionResponse{OK: false, Message: "Configurazione applicata ma non salvata su disco.", State: state}
	}
	return ActionResponse{OK: true, Message: "Configurazione salvata.", State: state}
}

// SetAsDefault rende le regole fornite il nuovo default (persistito in defaults.json).
// NON le applica come configurazione corrente: quella si salva a parte con SetConfig.
func (a *App) SetAsDefault(cfg rules.Config) ActionResponse {
	cfg = normalizeConfig(cfg)
	defErr := settings.SaveDefaults(cfg)

	a.mu.Lock()
	a.defaults = cfg
	if defErr != nil {
		a.addLogLocked(LogError, "Default aggiornati ma NON salvati su disco: "+defErr.Error())
	} else {
		a.addLogLocked(LogSuccess, "Nuovo default salvato.")
	}
	state := a.snapshot()
	a.mu.Unlock()

	if defErr != nil {
		return ActionResponse{OK: false, Message: "Default non salvati su disco.", State: state}
	}
	return ActionResponse{OK: true, Message: "Nuovo default salvato.", State: state}
}

// ResetConfig ripristina la configurazione corrente ai default persistiti e la salva.
func (a *App) ResetConfig() ActionResponse {
	a.mu.Lock()
	cfg := a.defaults
	cfg.StartFolder = a.config.StartFolder // mantieni la cartella corrente
	a.mu.Unlock()

	saveErr := settings.SaveConfig(cfg)

	a.mu.Lock()
	a.config = cfg
	// NON azzeriamo scanned: le anteprime restano e si ricalcolano con le regole di default.
	if saveErr != nil {
		a.addLogLocked(LogError, "Default ripristinati ma NON salvati: "+saveErr.Error())
	} else {
		a.addLogLocked(LogSuccess, "Configurazione ripristinata ai default e salvata.")
	}
	watchWanted := a.watchEnabled
	state := a.snapshot()
	a.mu.Unlock()

	if watchWanted {
		_ = a.startWatcher()
	}

	if saveErr != nil {
		return ActionResponse{OK: false, Message: "Default ripristinati ma non salvati su disco.", State: state}
	}
	return ActionResponse{OK: true, Message: "Configurazione ripristinata ai default e salvata.", State: state}
}

func (a *App) Scan() ActionResponse {
	files, err := rename.NewService(a.currentConfig()).Scan()
	if err != nil {
		return ActionResponse{OK: false, Message: "Errore scansione: " + err.Error(), State: a.snapshotLocked()}
	}

	a.mu.Lock()
	a.scanned = files
	a.watchPaused = false
	a.addLogLocked(LogInfo, fmt.Sprintf("Scansione completata: %d file audio.", len(files)))
	state := a.snapshot()
	a.mu.Unlock()

	return ActionResponse{OK: true, Message: "Scansione completata.", State: state}
}

// ChooseDirectory apre il selettore cartella e restituisce il percorso scelto
// (stringa vuota se annullato). Usato per selezionare la cartella di destinazione.
func (a *App) ChooseDirectory() string {
	path, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Seleziona cartella di destinazione",
	})
	if err != nil {
		return ""
	}
	return path
}

// ProcessAll esegue in un colpo solo normalizzazione dei nomi + scrittura tag.
// destination vuota => stessa cartella di partenza. deleteOriginals=false scrive
// una copia lasciando intatti gli originali (e gli altri file presenti).
func (a *App) ProcessAll() ActionResponse {
	a.mu.Lock()
	cfg := a.config
	destSame := a.destSameAsSource
	destFolder := a.destFolder
	deleteOriginals := a.deleteOriginals
	files := append([]string(nil), a.scanned...)
	a.mu.Unlock()

	destination := ""
	if !destSame {
		if destFolder == "" {
			return ActionResponse{OK: false, Message: "Scegli una cartella di destinazione.", State: a.snapshotLocked()}
		}
		destination = destFolder
	}
	if destination != "" && !appfs.IsDir(destination) {
		return ActionResponse{OK: false, Message: "La cartella di destinazione non esiste.", State: a.snapshotLocked()}
	}

	service := rename.NewService(cfg)
	if files == nil {
		var err error
		files, err = service.Scan()
		if err != nil {
			return ActionResponse{OK: false, Message: "Errore scansione: " + err.Error(), State: a.snapshotLocked()}
		}
	}

	results, err := service.Process(files, rename.Options{
		DestinationFolder: destination,
		DeleteOriginals:   deleteOriginals,
	})
	if err != nil {
		return ActionResponse{OK: false, Message: "Errore elaborazione: " + err.Error(), State: a.snapshotLocked()}
	}

	tagged, skipped := 0, 0
	views := make([]ResultView, 0, len(results))
	for _, result := range results {
		if result.Tagged {
			tagged++
		}
		if result.Skipped {
			skipped++
		}
		views = append(views, ResultView{
			OldName: filepath.Base(result.OldPath),
			NewName: result.NewName,
			Tagged:  result.Tagged,
			Skipped: result.Skipped,
			Reason:  result.Reason,
		})
	}

	a.mu.Lock()
	a.scanned = nil
	// Da qui in avanti l'UI mostra "Avvia nuova scansione": ignoriamo gli eventi
	// del watcher (compresi quelli auto-generati da questa elaborazione) fino
	// alla prossima Scan/cambio cartella.
	a.watchPaused = true
	a.addLogLocked(LogSuccess, fmt.Sprintf("Elaborati %d file (%d con tag MP3).", len(results)-skipped, tagged))
	if skipped > 0 {
		if deleteOriginals {
			a.addLogLocked(LogInfo, fmt.Sprintf("Saltati ed eliminati %d file.", skipped))
		} else {
			a.addLogLocked(LogInfo, fmt.Sprintf("Saltati %d file.", skipped))
		}
	}
	state := a.snapshot()
	a.mu.Unlock()

	return ActionResponse{OK: true, Message: "Elaborazione completata.", State: state, Results: views}
}

// SetWatchEnabled attiva o disattiva la modalità watch (elaborazione automatica
// dei nuovi file che compaiono nella cartella sorgente). Lo stato è persistito
// e ripristinato al prossimo avvio.
func (a *App) SetWatchEnabled(enabled bool) ActionResponse {
	a.mu.Lock()
	folder := a.config.StartFolder
	a.mu.Unlock()

	if enabled && !appfs.IsDir(folder) {
		return ActionResponse{OK: false, Message: "Seleziona prima una cartella valida.", State: a.snapshotLocked()}
	}

	var (
		startErr error
		message  string
	)
	if enabled {
		startErr = a.startWatcher()
	} else {
		a.stopWatcher()
	}

	a.mu.Lock()
	if enabled && startErr != nil {
		a.watchEnabled = false
		a.addLogLocked(LogError, "Impossibile avviare l'aggiornamento automatico: "+startErr.Error())
		message = "Impossibile avviare l'aggiornamento automatico."
	} else {
		a.watchEnabled = enabled
		if enabled {
			a.addLogLocked(LogAuto, "Aggiornamento automatico attivato su: "+folder)
			message = "Aggiornamento automatico attivato."
		} else {
			a.addLogLocked(LogAuto, "Aggiornamento automatico disattivato.")
			message = "Aggiornamento automatico disattivato."
		}
	}
	a.persistStateLocked()
	a.mu.Unlock()

	// All'attivazione facciamo una scansione immediata: la cartella potrebbe
	// essere cambiata prima che l'utente cliccasse il toggle, e senza questa
	// scansione l'anteprima si aggiornerebbe solo al primo evento successivo.
	if enabled && startErr == nil {
		if files, err := rename.NewService(a.currentConfig()).Scan(); err == nil {
			a.mu.Lock()
			a.scanned = files
			a.watchPaused = false
			a.addLogLocked(LogAuto, fmt.Sprintf("Scansione iniziale: %d file audio.", len(files)))
			a.mu.Unlock()
		} else {
			a.mu.Lock()
			a.addLogLocked(LogError, "Scansione iniziale fallita: "+err.Error())
			a.mu.Unlock()
		}
	}

	return ActionResponse{OK: startErr == nil, Message: message, State: a.snapshotLocked()}
}

// startWatcher avvia l'osservazione della cartella sorgente corrente.
// Se già attivo, viene riavviato (utile dopo cambio cartella o regole).
func (a *App) startWatcher() error {
	a.mu.Lock()
	folder := a.config.StartFolder
	cfg := a.config
	w := a.watcher
	a.mu.Unlock()

	if w == nil {
		return nil
	}
	return w.Start(folder, cfg, a.onWatchFile, a.onWatchError)
}

func (a *App) stopWatcher() {
	a.mu.Lock()
	w := a.watcher
	if a.watchDebounce != nil {
		a.watchDebounce.Stop()
		a.watchDebounce = nil
	}
	a.mu.Unlock()
	if w != nil {
		w.Stop()
	}
}

// watchRescanDebounce è la finestra di quiete richiesta prima di eseguire un
// rescan+notify globale. Coalizza raffiche di eventi su file DIVERSI (es. copia
// massiva di più file) in un'unica scansione. Il watcher applica già un debounce
// per-file di pari durata (watcher.DefaultQuietPeriod): i due lavorano in
// cascata e sono deliberatamente distinti, quindi cambiare uno non impatta l'altro.
const watchRescanDebounce = 150 * time.Millisecond

// onWatchFile viene invocato dal watcher quando cambia il contenuto della
// cartella osservata (nuovo file, modifica o rimozione). NON esegue conversioni:
// si limita a schedulare (con debounce) una scansione + notifica all'UI, che
// aggiornerà l'anteprima. Se il watcher è in pausa (post-ProcessAll, prima di
// una nuova Scan manuale) l'evento viene ignorato: è così che filtriamo
// automaticamente gli eventi fsnotify generati dalla conversione appena
// eseguita.
func (a *App) onWatchFile(_ string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.watcher == nil || !a.watchEnabled || a.watchPaused {
		return
	}
	if a.watchDebounce != nil {
		a.watchDebounce.Reset(watchRescanDebounce)
		return
	}
	a.watchDebounce = time.AfterFunc(watchRescanDebounce, a.runWatchRescan)
}

// runWatchRescan esegue la scansione differita e notifica l'UI. Chiamata dal
// timer di debounce; se nel frattempo il watcher è stato fermato o messo in
// pausa, non fa nulla.
func (a *App) runWatchRescan() {
	a.mu.Lock()
	a.watchDebounce = nil
	if a.watcher == nil || !a.watchEnabled || a.watchPaused {
		a.mu.Unlock()
		return
	}
	cfg := a.config
	a.mu.Unlock()

	files, err := rename.NewService(cfg).Scan()
	if err != nil {
		a.mu.Lock()
		a.addLogLocked(LogError, "Aggiornamento automatico: errore scansione: "+err.Error())
		a.mu.Unlock()
		return
	}

	a.mu.Lock()
	a.scanned = files
	a.addLogLocked(LogAuto, fmt.Sprintf("Scansione automatica: %d file audio.", len(files)))
	state := a.snapshot()
	ctx := a.ctx
	a.mu.Unlock()

	if ctx != nil {
		wailsruntime.EventsEmit(ctx, EventWatchChanged, state)
	}
}

func (a *App) onWatchError(err error) {
	a.mu.Lock()
	a.addLogLocked(LogError, "Aggiornamento automatico: errore filesystem: "+err.Error())
	a.mu.Unlock()
}

func (a *App) currentConfig() rules.Config {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.config
}

func (a *App) snapshotLocked() StateResponse {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.snapshot()
}

func (a *App) snapshot() StateResponse {
	files := make([]FileView, 0, len(a.scanned))
	for _, path := range a.scanned {
		name := filepath.Base(path)
		ext := parser.Extension(name)
		preview := a.config.NormalizeFileBase(parser.RemoveExtension(name)) + "." + ext
		files = append(files, FileView{
			Name:    name,
			Path:    path,
			Preview: preview,
			MP3:     ext == "mp3",
		})
	}
	logs := append([]LogEntry(nil), a.logs...)
	return StateResponse{
		Folder:                  a.config.StartFolder,
		Files:                   files,
		Logs:                    logs,
		Config:                  a.config,
		DestinationSameAsSource: a.destSameAsSource,
		DestinationFolder:       a.destFolder,
		DeleteOriginals:         a.deleteOriginals,
		WatchEnabled:            a.watchEnabled,
		WatchActive:             a.watcher != nil && a.watcher.Folder() != "",
	}
}

func (a *App) addLogLocked(kind LogKind, message string) {
	a.logs = append([]LogEntry{newLogEntry(kind, message)}, a.logs...)
	if len(a.logs) > 12 {
		a.logs = a.logs[:12]
	}
}

// newLogEntry costruisce una riga di attività con l'orario corrente. Isolata in
// una funzione così può essere usata anche nei bootstrap (NewApp) dove non c'è
// ancora l'istanza App.
func newLogEntry(kind LogKind, message string) LogEntry {
	return LogEntry{
		Time:    time.Now().Format("15:04:05"),
		Kind:    kind,
		Message: message,
	}
}

// normalizeConfig ripulisce la configurazione ricevuta dalla GUI: trim del percorso,
// rimozione delle voci vuote nelle liste e delle sostituzioni senza From.
func normalizeConfig(cfg rules.Config) rules.Config {
	cfg.StartFolder = strings.Trim(cfg.StartFolder, `" `)
	cfg.SupportedExtensions = cleanList(cfg.SupportedExtensions)
	cfg.OccurrenciesToRemove = cleanList(cfg.OccurrenciesToRemove)
	cfg.OccurrenciesToReplaceWithFt = cleanList(cfg.OccurrenciesToReplaceWithFt)

	replacements := make([]rules.Replacement, 0, len(cfg.Replacements))
	for _, r := range cfg.Replacements {
		if strings.TrimSpace(r.From) == "" {
			continue
		}
		replacements = append(replacements, r)
	}
	cfg.Replacements = replacements
	return cfg
}

func cleanList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) == "" {
			continue
		}
		out = append(out, v)
	}
	return out
}
