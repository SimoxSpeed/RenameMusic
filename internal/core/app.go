// Package core è il cuore applicativo di RenameMusic, indipendente dalla
// piattaforma: stato (cartella, regole, ultimo scan, log, opzioni) e metodi
// chiamati dalla UI. Lo usano sia l'app desktop (Wails, package main) sia l'app
// Android (package mobile, via gomobile): le differenze di piattaforma passano
// dalle interfacce Host e YtDlp (vedi platform.go).
package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	appfs "renamemusic/internal/fs"
	"renamemusic/internal/parser"
	"renamemusic/internal/playlist"
	"renamemusic/internal/rename"
	"renamemusic/internal/rules"
	"renamemusic/internal/settings"
	"renamemusic/internal/update"
	"renamemusic/internal/watcher"
)

// EventWatchChanged è l'evento emesso quando la modalità watch rileva
// una variazione nella cartella osservata: il payload è lo StateResponse
// aggiornato (con la nuova lista di file/anteprime), così l'UI può aggiornare
// solo l'anteprima senza avviare nulla — la conversione resta sempre manuale.
const EventWatchChanged = "watch:changed"

// EventFolderDropped è l'evento emesso quando l'utente trascina una
// cartella (o un file) sulla finestra: il payload è lo StateResponse aggiornato
// con la nuova cartella di partenza e la relativa anteprima.
const EventFolderDropped = "folder:dropped"

// EventProcessProgress è emesso durante ProcessAll dopo ogni file elaborato, con
// l'avanzamento corrente (done/total), così la UI può mostrare "(x/totale)".
const EventProcessProgress = "process:progress"

// EventYtDlpChanged è emesso quando lo stato di yt-dlp cambia fuori dal ciclo
// richiesta/risposta della UI (su Android: fine dell'inizializzazione di
// youtubedl-android, che avviene in background all'avvio). Il payload è lo
// StateResponse aggiornato.
const EventYtDlpChanged = "ytdlp:changed"

// Fasi di ProgressEvent: DownloadAndProcess emette l'avanzamento prima del
// download e poi della conversione, e la UI le distingue da qui.
const (
	PhaseDownload = "download"
	PhaseConvert  = "convert"
)

// ProgressEvent è il payload di EventProcessProgress. Phase è PhaseDownload o
// PhaseConvert (vuota per le altre operazioni, es. ClearTags).
type ProgressEvent struct {
	Done  int    `json:"done"`
	Total int    `json:"total"`
	Phase string `json:"phase,omitempty"`
}

// EventInstallProgress è emesso durante InstallYtDlp/InstallFFmpeg con
// l'avanzamento in byte del download (e, per ffmpeg, dell'estrazione), così la
// UI può mostrare la percentuale.
const EventInstallProgress = "install:progress"

// InstallProgressEvent è il payload di EventInstallProgress. Tool è "yt-dlp" o
// "ffmpeg"; Phase è playlist.PhaseDownload o playlist.PhaseExtract; Total <= 0
// se la dimensione non è nota.
type InstallProgressEvent struct {
	Tool  string `json:"tool"`
	Phase string `json:"phase"`
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
}

// installProgress restituisce il callback che inoltra alla UI l'avanzamento
// dell'installazione di `tool`.
func (a *App) installProgress(tool string) playlist.Progress {
	return func(phase string, done, total int64) {
		a.emit(EventInstallProgress, InstallProgressEvent{Tool: tool, Phase: phase, Done: done, Total: total})
	}
}

// TagPromptView descrive una traccia i cui tag, una volta normalizzato il nome,
// risulterebbero sconosciuti (titolo o artista). ProcessAll ne restituisce
// l'elenco nella risposta (campo Prompts di ActionResponse): la UI mostra un
// popup per ciascuna — con OriginalBase modificabile — e la risolve chiamando
// ResolveTagPrompt, mentre le tracce a posto sono già state convertite. NON si
// blocca nulla lato backend: ogni scelta è una chiamata a sé.
//
// OriginalBase è il nome (senza estensione) da mostrare/modificare; Ext serve
// alla UI per il chip di formato; Title/Artist sono i tag dedotti (uno dei due,
// o entrambi, sarà "sconosciuto").
type TagPromptView struct {
	Path         string `json:"path"`
	OriginalBase string `json:"originalBase"`
	Ext          string `json:"ext"`
	Title        string `json:"title"`
	Artist       string `json:"artist"`
	// Review: traccia rinominabile che l'utente ha scelto di rivedere
	// dall'anteprima (non ha tag sconosciuti). PreviewBase è il nome proposto
	// dall'anteprima, da cui il popup parte al posto del nome originale.
	Review      bool   `json:"review,omitempty"`
	PreviewBase string `json:"previewBase,omitempty"`
}

type App struct {
	// host e ytdlp sono i servizi di piattaforma (vedi platform.go), fissati
	// da New e mai modificati dopo: si leggono senza lock tramite h() e yt().
	host    Host
	ytdlp   YtDlp
	updater Updater

	mu       sync.Mutex
	config   rules.Config
	defaults rules.Config
	scanned  []string
	// currentTags tiene, per ogni percorso mp3 in scanned, i tag ID3
	// attualmente letti dal file. Calcolato una sola volta insieme alla
	// scansione (I/O su disco), non ad ogni snapshot; il tag che verrebbe
	// invece scritto è ricavato al volo in snapshot() dal nome normalizzato,
	// senza bisogno di I/O.
	currentTags map[string]rename.TagInfo
	logs        []LogEntry

	// playlists sono le playlist YouTube salvate (nome -> link), gestite in
	// Impostazioni e usate da DownloadPlaylist. Persistite a parte
	// (playlists.json): non sono regole di rinomina.
	//
	// defaultPlaylists è il preset predefinito delle playlist (defaults-playlists.json),
	// analogo a `defaults` per le regole: "Salva come predefinito" lo aggiorna,
	// "Ripristina predefiniti" lo ricopia in playlists.
	playlists        []playlist.Playlist
	defaultPlaylists []playlist.Playlist

	// Opzioni di elaborazione persistite (state.json).
	destSameAsSource bool
	destFolder       string
	deleteOriginals  bool
	watchEnabled     bool

	// Gestione di yt-dlp (state.json). ytDlpManaged: l'app usa/aggiorna la sua
	// copia in %AppData%\RenameMusic; se false si usa ytDlpPath (scelto a mano).
	// ytDlpAvailable/ytDlpVersion sono una cache dello stato del percorso
	// effettivo, ricalcolata da refreshYtDlpStatus (che stat-a il file ed esegue
	// `yt-dlp --version`): così snapshot() non lancia un processo ad ogni chiamata.
	ytDlpManaged   bool
	ytDlpPath      string
	ytDlpAvailable bool
	ytDlpVersion   string
	// ffmpegAvailable: cache di YtDlp.FFmpegAvailable (ffmpeg serve a yt-dlp
	// per l'mp3), ricalcolata insieme allo stato di yt-dlp.
	ffmpegAvailable bool

	watcher *watcher.Watcher

	// watchPaused sospende le notifiche del watcher finché non arriva un nuovo
	// Scan (o cambio cartella). Viene alzato al termine di un ProcessAll: in UI
	// coincide con la fase in cui è visibile il tasto "Avvia nuova scansione",
	// ed è il modo più semplice per ignorare gli eventi fsnotify che noi stessi
	// abbiamo appena generato.
	watchPaused bool

	// watchDebounce coalisce eventi ravvicinati in un unico rescan globale.
	watchDebounce *time.Timer

	// opCancel è la funzione di cancellazione dell'operazione lunga in corso
	// (ProcessAll o ClearTags), o nil se nessuna è attiva. La invoca Cancel().
	opCancel context.CancelFunc

	// Aggiornamenti dell'app (vedi update.go). latestRelease/latestAsset: la
	// release più recente di quella in uso, se trovata dall'ultimo controllo
	// (nil altrimenti). updateSeen: versione per cui il popup è già stato
	// mostrato (state.json). updateInstalling impedisce due installazioni
	// insieme; checkMu serializza i controlli (loop periodico e manuali).
	latestRelease    *update.Release
	latestAsset      update.Asset
	updateSeen       string
	updateInstalling bool
	checkMu          sync.Mutex

	// ytDlpMu è tenuto mentre yt-dlp è in uso (download di una playlist) o viene
	// sostituito (installazione, aggiornamento automatico): l'aggiornamento
	// automatico non deve cambiarlo sotto a un download (vedi ytdlp_update.go).
	// ytDlpCheck anticipa il prossimo controllo automatico.
	ytDlpMu    sync.Mutex
	ytDlpCheck chan struct{}
}

type FileView struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Preview string `json:"preview"`
	MP3     bool   `json:"mp3"`
	// Title/Artist sono i tag ID3 attualmente presenti sul file (vuoti se
	// assenti o illeggibili); TitlePreview/ArtistPreview sono quelli che
	// ProcessAll scriverebbe. Valorizzati solo per MP3; la UI li confronta per
	// evidenziare le differenze, come già fa per Name/Preview.
	Title         string `json:"title,omitempty"`
	Artist        string `json:"artist,omitempty"`
	TitlePreview  string `json:"titlePreview,omitempty"`
	ArtistPreview string `json:"artistPreview,omitempty"`
}

type ResultView struct {
	OldName  string `json:"oldName"`
	NewName  string `json:"newName"`
	Tagged   bool   `json:"tagged"`
	Skipped  bool   `json:"skipped"`
	Failed   bool   `json:"failed"`
	Canceled bool   `json:"canceled"`
	Reason   string `json:"reason"`
	// MP3 indica se il file è un MP3 (unico formato per cui si scrivono i tag);
	// Title/Artist sono i tag ID3 scritti da ProcessAll, ricavati dal nuovo nome
	// come in FileView, così la UI può mostrarli in colonna anche nel risultato
	// della conversione, esattamente come già fa nell'anteprima.
	MP3    bool   `json:"mp3"`
	Title  string `json:"title,omitempty"`
	Artist string `json:"artist,omitempty"`
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
	// FolderMissing / DestinationMissing: la cartella di partenza (o quella di
	// destinazione, se distinta) è impostata ma non si trova sul disco. La UI
	// avverte e non permette scansione, download e conversione.
	FolderMissing      bool                `json:"folderMissing"`
	DestinationMissing bool                `json:"destinationMissing"`
	WatchEnabled       bool                `json:"watchEnabled"`
	WatchActive        bool                `json:"watchActive"`
	Playlists          []playlist.Playlist `json:"playlists"`
	YtDlpManaged       bool                `json:"ytDlpManaged"`
	YtDlpPath          string              `json:"ytDlpPath"`
	YtDlpEffectivePath string              `json:"ytDlpEffectivePath"`
	YtDlpAvailable     bool                `json:"ytDlpAvailable"`
	YtDlpVersion       string              `json:"ytDlpVersion"`
	FFmpegAvailable    bool                `json:"ffmpegAvailable"`
	AppVersion         string              `json:"appVersion"`
	// Update è la nuova versione disponibile (assente se l'app è aggiornata o
	// il controllo non è ancora riuscito).
	Update *UpdateView `json:"update,omitempty"`
}

// DownloadErrorView descrive un singolo video di playlist il cui download è
// fallito: la UI ne fa l'elenco in un modale dedicato dopo un DownloadPlaylist
// concluso con errori. Title può essere vuoto (dipende da yt-dlp).
type DownloadErrorView struct {
	VideoID string `json:"videoId"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Message string `json:"message"`
}

type ActionResponse struct {
	OK      bool          `json:"ok"`
	Message string        `json:"message"`
	State   StateResponse `json:"state"`
	Results []ResultView  `json:"results,omitempty"`
	// Prompts elenca le tracce che, una volta normalizzate, avrebbero tag
	// sconosciuti: ProcessAll le restituisce qui (senza convertirle) perché la UI
	// chieda all'utente come procedere, una alla volta, via ResolveTagPrompt.
	Prompts []TagPromptView `json:"prompts,omitempty"`
	// DownloadErrors elenca i video non scaricati (con dettaglio) dopo un
	// DownloadPlaylist con errori, così la UI può mostrarli in un modale.
	DownloadErrors []DownloadErrorView `json:"downloadErrors,omitempty"`
}

// Options configura i servizi di piattaforma di un App. Campi nil => Host che
// scarta gli eventi, gestione di yt-dlp desktop (ExecYtDlp) e nessun controllo
// degli aggiornamenti.
type Options struct {
	Host    Host
	YtDlp   YtDlp
	Updater Updater
}

// New crea l'App caricando regole, predefiniti, stato e playlist persistiti.
// Va poi avviato con Start.
func New(opts Options) *App {
	logs := []LogEntry{newLogEntry(LogInfo, "App pronta.")}

	// I predefiniti sono persistiti: se il file non esiste lo si crea dal seed di fabbrica.
	defaults, defExisted, err := settings.LoadDefaults()
	if err != nil {
		logs = []LogEntry{newLogEntry(LogError, "Impossibile leggere i predefiniti salvati: uso il seed di fabbrica.")}
	}
	if !defExisted {
		_ = settings.SaveDefaults(defaults)
	}

	// Le regole correnti: se il file non esiste si inizializzano dai predefiniti.
	current, curExisted, err := settings.LoadConfig()
	if err != nil {
		logs = []LogEntry{newLogEntry(LogError, "Impossibile leggere la configurazione salvata: uso i predefiniti.")}
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

	// Le playlist YouTube sono persistite a parte (non sono regole di rinomina),
	// con un preset predefinito analogo ai default delle regole.
	playlists, err := settings.LoadPlaylists()
	if err != nil {
		logs = append([]LogEntry{newLogEntry(LogError, "Impossibile leggere le playlist salvate.")}, logs...)
	}
	defaultPlaylists, err := settings.LoadDefaultPlaylists()
	if err != nil {
		logs = append([]LogEntry{newLogEntry(LogError, "Impossibile leggere le playlist predefinite.")}, logs...)
	}

	w := watcher.New()
	if os.Getenv("RENAMEMUSIC_WATCH_DEBUG") != "" {
		w.Debug = true
	}

	return &App{
		host:             opts.Host,
		ytdlp:            opts.YtDlp,
		updater:          opts.Updater,
		updateSeen:       st.UpdateSeenVersion,
		config:           current,
		defaults:         defaults,
		logs:             logs,
		playlists:        playlists,
		defaultPlaylists: defaultPlaylists,
		destSameAsSource: st.DestinationSameAsSource,
		destFolder:       st.DestinationFolder,
		deleteOriginals:  st.DeleteOriginals,
		watchEnabled:     st.WatchEnabled,
		ytDlpManaged:     st.YtDlpManaged,
		ytDlpPath:        st.YtDlpPath,
		ytDlpCheck:       make(chan struct{}, 1),
		watcher:          w,
	}
}

// h restituisce l'Host di piattaforma (nopHost se non fornito, es. nei test).
func (a *App) h() Host {
	if a.host == nil {
		return nopHost{}
	}
	return a.host
}

// yt restituisce la gestione di yt-dlp (ExecYtDlp se non fornita).
func (a *App) yt() YtDlp {
	if a.ytdlp == nil {
		return ExecYtDlp{}
	}
	return a.ytdlp
}

// emit invia un evento alla UI tramite l'Host.
func (a *App) emit(event string, payload any) {
	a.h().Emit(event, payload)
}

// persistStateLocked salva su disco cartella + opzioni. Va chiamata con il lock acquisito.
func (a *App) persistStateLocked() {
	_ = settings.SaveState(settings.State{
		LastFolder:              a.config.StartFolder,
		DestinationSameAsSource: a.destSameAsSource,
		DestinationFolder:       a.destFolder,
		DeleteOriginals:         a.deleteOriginals,
		WatchEnabled:            a.watchEnabled,
		YtDlpManaged:            a.ytDlpManaged,
		YtDlpPath:               a.ytDlpPath,
		UpdateSeenVersion:       a.updateSeen,
	})
}

// ytDlpEffectivePath restituisce il percorso di yt-dlp effettivamente in uso: la
// copia gestita in %AppData%\RenameMusic se "gestisci autonomamente" è attivo,
// altrimenti il percorso personalizzato scelto dall'utente. Va chiamata con il
// lock acquisito (legge ytDlpManaged/ytDlpPath).
func (a *App) ytDlpEffectivePath() string {
	if a.ytDlpManaged {
		if p, err := a.yt().ManagedPath(); err == nil {
			return p
		}
		return ""
	}
	return a.ytDlpPath
}

// refreshYtDlpStatus ricalcola la cache presenza/versione dal percorso effettivo.
// Esegue un `yt-dlp --version`, quindi NON va chiamata su ogni snapshot ma solo
// quando lo stato può essere cambiato (avvio, cambio configurazione, download).
// Va chiamata con il lock acquisito (o durante la costruzione, senza concorrenza).
func (a *App) refreshYtDlpStatus() {
	path := a.ytDlpEffectivePath()
	a.ytDlpAvailable = a.yt().Available(path)
	if a.ytDlpAvailable {
		a.ytDlpVersion = a.yt().Version(path)
	} else {
		a.ytDlpVersion = ""
	}
	a.ffmpegAvailable = a.yt().FFmpegAvailable()
}

// Start esegue le operazioni di avvio (pulizia temporanei, stato di yt-dlp,
// ripristino dell'aggiornamento automatico). È una funzione di package e non un
// metodo di proposito: i metodi esportati di App sono l'API della UI (bindata da
// Wails), mentre questo è un hook di ciclo di vita riservato alla piattaforma.
func Start(a *App) {
	// Pulizia dei file temporanei di configurazione lasciati orfani da un crash
	// durante una scrittura atomica (%AppData%\RenameMusic).
	if n := settings.CleanTempFiles(); n > 0 {
		a.mu.Lock()
		a.addLogLocked(LogInfo, fmt.Sprintf("Rimossi %d file temporanei di configurazione residui.", n))
		a.mu.Unlock()
	}

	// Rileva presenza/versione di yt-dlp una volta all'avvio, così il primo
	// GetState riporta già lo stato del "bin" senza attese in UI.
	a.mu.Lock()
	a.refreshYtDlpStatus()
	a.mu.Unlock()

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

	// Controllo periodico delle nuove versioni dell'app, in background: se non
	// c'è connessione riprova finché non torna (vedi updateLoop).
	if a.updater != nil {
		go a.updateLoop()
	}

	// Aggiornamento automatico della copia di yt-dlp gestita dall'app.
	go a.ytDlpUpdateLoop()
}

// RefreshYtDlp ricalcola lo stato di yt-dlp e notifica la UI con
// EventYtDlpChanged. Serve alle piattaforme in cui yt-dlp diventa disponibile
// in modo asincrono (Android: inizializzazione di youtubedl-android).
func RefreshYtDlp(a *App) {
	a.mu.Lock()
	a.refreshYtDlpStatus()
	state := a.snapshot()
	a.mu.Unlock()
	a.emit(EventYtDlpChanged, state)
	// Su Android yt-dlp diventa disponibile solo adesso: il controllo fatto
	// all'avvio l'ha trovato non pronto.
	a.requestYtDlpCheck()
}

// NotifyExternalChange segnala che il contenuto della cartella potrebbe essere
// cambiato senza che il watcher lo abbia visto (su Android: l'app torna in primo
// piano dopo che l'utente ha usato un altro file manager). Se l'aggiornamento
// automatico è attivo (e non in pausa) programma la stessa scansione+notifica
// di un evento fsnotify; altrimenti non fa nulla.
func NotifyExternalChange(a *App) {
	a.onWatchFile("")
}

// HandleFileDrop gestisce il rilascio di elementi sulla finestra: usa il primo
// percorso (se è un file, risale alla cartella che lo contiene), imposta la
// cartella di partenza riusando SetFolder e notifica la UI con l'anteprima
// aggiornata. Percorsi multipli: si considera solo il primo (l'app lavora su una
// cartella per volta).
func HandleFileDrop(a *App, paths []string) {
	if len(paths) == 0 {
		return
	}

	folder := paths[0]
	if !appfs.IsDir(folder) {
		folder = filepath.Dir(folder)
	}

	resp := a.SetFolder(folder)
	if len(paths) > 1 {
		a.mu.Lock()
		a.addLogLocked(LogInfo, fmt.Sprintf("Trascinati %d elementi: uso solo il primo.", len(paths)))
		resp.State = a.snapshot()
		a.mu.Unlock()
	}

	a.emit(EventFolderDropped, resp.State)
}

// OpenFolder apre il percorso indicato nel file manager di sistema (Esplora
// risorse su Windows). Nota: explorer.exe restituisce spesso exit code 1 anche
// quando apre correttamente, quindi l'errore di avvio non è trattato come
// fallimento.
func (a *App) OpenFolder(path string) ActionResponse {
	path = strings.Trim(path, `" `)
	if !appfs.IsDir(path) {
		return ActionResponse{OK: false, Message: "La cartella non esiste.", State: a.snapshotLocked()}
	}
	// Su Windows l'Host apre la cartella tramite la shell già in esecuzione
	// (ShellExecuteW): quasi istantanea, niente cold-start di explorer.exe.
	if err := a.h().OpenFolder(filepath.Clean(path)); err != nil {
		return ActionResponse{OK: false, Message: "Impossibile aprire la cartella: " + err.Error(), State: a.snapshotLocked()}
	}
	return ActionResponse{OK: true, State: a.snapshotLocked()}
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
// In modalità semplificata non scansiona: non c'è un'anteprima da popolare, e
// "Scarica e converti" riscansiona comunque dopo il download (tornando alla
// modalità normale ci pensa la riscansione di SetConfig).
func (a *App) ensureScanned() string {
	a.mu.Lock()
	needScan := a.scanned == nil && !a.config.SimpleMode && a.config.StartFolder != ""
	cfg := a.config
	a.mu.Unlock()
	if !needScan {
		return ""
	}
	if msg := a.foldersError(); msg != "" {
		a.mu.Lock()
		a.addLogLocked(LogError, msg)
		a.mu.Unlock()
		return msg
	}

	files, err := rename.NewService(cfg).Scan()
	if err != nil {
		a.mu.Lock()
		a.addLogLocked(LogError, "Errore scansione iniziale: "+err.Error())
		a.mu.Unlock()
		return "Errore scansione: " + err.Error()
	}
	currentTags := currentTagsFor(files)

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.scanned != nil {
		return "" // una scansione concorrente ha già valorizzato lo stato
	}
	a.scanned = files
	a.currentTags = currentTags
	a.watchPaused = false
	a.addLogLocked(LogInfo, fmt.Sprintf("Scansione completata: %d file audio.", len(files)))
	return "Scansione completata."
}

// currentTagsFor legge, per ogni file mp3 scansionato, i tag ID3 attualmente
// presenti (non dipende dalle regole configurate). Fa I/O su disco: va
// chiamata SENZA lock, insieme alla scansione stessa; il risultato va poi
// assegnato sotto lock come a.currentTags.
func currentTagsFor(files []string) map[string]rename.TagInfo {
	info := make(map[string]rename.TagInfo, len(files))
	for _, path := range files {
		if parser.Extension(filepath.Base(path)) != "mp3" {
			continue
		}
		info[path] = rename.CurrentTags(path)
	}
	return info
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
	a.currentTags = nil
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
	path, err := a.h().ChooseDirectory("Seleziona cartella Rename Music")
	if err != nil {
		return ActionResponse{OK: false, Message: "Impossibile aprire il selettore cartella.", State: a.snapshotLocked()}
	}
	if path == "" {
		return ActionResponse{OK: false, Message: "Selezione annullata.", State: a.snapshotLocked()}
	}
	return a.SetFolder(path)
}

// SetConfig applica una nuova configurazione di regole/cartella (correnti) e la salva su disco.
// Le voci vuote vengono scartate. Non tocca i predefiniti.
func (a *App) SetConfig(cfg rules.Config) ActionResponse {
	cfg = normalizeConfig(cfg)

	a.mu.Lock()
	cfg.StartFolder = a.config.StartFolder // la cartella si gestisce a parte, non la tocchiamo
	a.mu.Unlock()

	saveErr := settings.SaveConfig(cfg)

	// Riscansiona con le nuove regole (I/O fuori dal lock): le regole possono
	// cambiare non solo il nome normalizzato ma anche QUALI file rientrano
	// (estensioni supportate/occorrenze), e dopo un ProcessAll `scanned` è nil.
	files, currentTags, rescanned, scanErr := a.rescanWith(cfg)

	a.mu.Lock()
	a.config = cfg
	if rescanned {
		a.scanned = files
		a.currentTags = currentTags
		a.watchPaused = false
	}
	if saveErr != nil {
		a.addLogLocked(LogError, "Configurazione applicata ma NON salvata: "+saveErr.Error())
	} else {
		a.addLogLocked(LogSuccess, "Configurazione salvata.")
	}
	if scanErr != nil {
		a.addLogLocked(LogError, "Scansione con le nuove regole fallita: "+scanErr.Error())
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

// SetAsDefault rende i nuovi predefiniti le regole fornite (persistite in
// defaults.json) e l'elenco di playlist fornito (defaults-playlists.json). NON li
// applica come configurazione/playlist correnti: quelle si salvano a parte con
// SetConfig/SetPlaylists.
func (a *App) SetAsDefault(cfg rules.Config, playlists []playlist.Playlist) ActionResponse {
	cfg = normalizeConfig(cfg)
	cleaned := cleanPlaylists(playlists)
	defErr := settings.SaveDefaults(cfg)
	plErr := settings.SaveDefaultPlaylists(cleaned)

	a.mu.Lock()
	a.defaults = cfg
	a.defaultPlaylists = cleaned
	saveErr := defErr
	if saveErr == nil {
		saveErr = plErr
	}
	if saveErr != nil {
		a.addLogLocked(LogError, "Predefiniti aggiornati ma NON salvati su disco: "+saveErr.Error())
	} else {
		a.addLogLocked(LogSuccess, "Nuovi predefiniti salvati.")
	}
	state := a.snapshot()
	a.mu.Unlock()

	if saveErr != nil {
		return ActionResponse{OK: false, Message: "Predefiniti non salvati su disco.", State: state}
	}
	return ActionResponse{OK: true, Message: "Nuovi predefiniti salvati.", State: state}
}

// ResetConfig ripristina ai predefiniti persistiti sia le regole correnti sia
// l'elenco di playlist, e li salva.
func (a *App) ResetConfig() ActionResponse {
	a.mu.Lock()
	cfg := a.defaults
	cfg.StartFolder = a.config.StartFolder // mantieni la cartella corrente
	playlists := cleanPlaylists(a.defaultPlaylists)
	a.mu.Unlock()

	saveErr := settings.SaveConfig(cfg)
	plErr := settings.SavePlaylists(playlists)

	// Riscansiona con le regole predefinite (vedi nota in SetConfig).
	files, currentTags, rescanned, scanErr := a.rescanWith(cfg)

	a.mu.Lock()
	a.config = cfg
	a.playlists = playlists
	if rescanned {
		a.scanned = files
		a.currentTags = currentTags
		a.watchPaused = false
	}
	if saveErr == nil {
		saveErr = plErr
	}
	if saveErr != nil {
		a.addLogLocked(LogError, "Predefiniti ripristinati ma NON salvati: "+saveErr.Error())
	} else {
		a.addLogLocked(LogSuccess, "Configurazione ripristinata ai predefiniti e salvata.")
	}
	if scanErr != nil {
		a.addLogLocked(LogError, "Scansione con le regole predefinite fallita: "+scanErr.Error())
	}
	watchWanted := a.watchEnabled
	state := a.snapshot()
	a.mu.Unlock()

	if watchWanted {
		_ = a.startWatcher()
	}

	if saveErr != nil {
		return ActionResponse{OK: false, Message: "Predefiniti ripristinati ma non salvati su disco.", State: state}
	}
	return ActionResponse{OK: true, Message: "Configurazione ripristinata ai predefiniti e salvata.", State: state}
}

func (a *App) Scan() ActionResponse {
	message, ok := a.performScan()
	return ActionResponse{OK: ok, Message: message, State: a.snapshotLocked()}
}

// performScan esegue una scansione con la config corrente e aggiorna lo stato
// (a.scanned/a.currentTags/a.watchPaused), loggando l'esito. Fa I/O su disco:
// va chiamata SENZA lock. Usata sia da Scan sia da DownloadPlaylist (che
// scansiona automaticamente a valle del download).
func (a *App) performScan() (message string, ok bool) {
	// Cartelle mancanti: niente scansione, e l'anteprima precedente (di una
	// cartella che non c'è più) si svuota.
	if msg := a.foldersError(); msg != "" {
		a.mu.Lock()
		a.scanned = nil
		a.currentTags = nil
		a.addLogLocked(LogError, msg)
		a.mu.Unlock()
		return msg, false
	}

	cfg := a.currentConfig()

	// Rimuove eventuali temporanei orfani da run precedenti prima di scansionare.
	if n := rename.CleanTempFiles(cfg.StartFolder); n > 0 {
		a.mu.Lock()
		a.addLogLocked(LogInfo, fmt.Sprintf("Rimossi %d file temporanei residui.", n))
		a.mu.Unlock()
	}

	files, err := rename.NewService(cfg).Scan()
	if err != nil {
		return "Errore scansione: " + err.Error(), false
	}
	currentTags := currentTagsFor(files)

	a.mu.Lock()
	a.scanned = files
	a.currentTags = currentTags
	a.watchPaused = false
	a.addLogLocked(LogInfo, fmt.Sprintf("Scansione completata: %d file audio.", len(files)))
	a.mu.Unlock()

	return "Scansione completata.", true
}

// foldersError restituisce il motivo per cui le cartelle correnti non sono
// utilizzabili ("" se lo sono): la cartella di partenza deve esistere, e così
// quella di destinazione se è distinta e già scelta (se manca del tutto lo
// segnala solo la conversione: l'anteprima si può vedere anche senza). Vale per
// scansione, download e conversione. Fa I/O su disco: va chiamata SENZA lock.
func (a *App) foldersError() string {
	a.mu.Lock()
	folder := a.config.StartFolder
	destSame := a.destSameAsSource
	destFolder := a.destFolder
	a.mu.Unlock()

	if folder == "" {
		return "Seleziona prima una cartella di partenza."
	}
	if !appfs.IsDir(folder) {
		return "Cartella di partenza non trovata: " + folder
	}
	if !destSame && destFolder != "" && !appfs.IsDir(destFolder) {
		return "Cartella di destinazione non trovata: " + destFolder
	}
	return ""
}

// cleanPlaylists normalizza un elenco di playlist scartando le voci senza nome o
// link (con trim su entrambi). Restituisce sempre uno slice non-nil.
func cleanPlaylists(list []playlist.Playlist) []playlist.Playlist {
	cleaned := make([]playlist.Playlist, 0, len(list))
	for _, p := range list {
		p.Name = strings.TrimSpace(p.Name)
		p.URL = strings.TrimSpace(p.URL)
		if p.Name == "" || p.URL == "" {
			continue
		}
		cleaned = append(cleaned, p)
	}
	return cleaned
}

// SetPlaylists sostituisce l'elenco delle playlist YouTube salvate (nome ->
// link) e lo persiste. Le voci senza nome o link vengono scartate.
func (a *App) SetPlaylists(list []playlist.Playlist) ActionResponse {
	cleaned := cleanPlaylists(list)

	saveErr := settings.SavePlaylists(cleaned)

	a.mu.Lock()
	a.playlists = cleaned
	if saveErr != nil {
		a.addLogLocked(LogError, "Playlist aggiornate ma NON salvate su disco: "+saveErr.Error())
	} else {
		a.addLogLocked(LogSuccess, "Playlist salvate.")
	}
	state := a.snapshot()
	a.mu.Unlock()

	if saveErr != nil {
		return ActionResponse{OK: false, Message: "Playlist non salvate su disco.", State: state}
	}
	return ActionResponse{OK: true, Message: "Playlist salvate.", State: state}
}

// SetYtDlpConfig aggiorna la modalità di gestione di yt-dlp: se `managed` è true
// l'app usa (e aggiorna) la propria copia in %AppData%\RenameMusic; altrimenti
// usa `path`, l'eseguibile yt-dlp scelto a mano dall'utente. Persiste su disco e
// ricalcola presenza/versione del percorso effettivo.
func (a *App) SetYtDlpConfig(managed bool, path string) ActionResponse {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ytDlpManaged = managed
	a.ytDlpPath = strings.TrimSpace(path)
	a.persistStateLocked()
	a.refreshYtDlpStatus()
	if managed {
		a.requestYtDlpCheck()
	}
	return ActionResponse{OK: true, State: a.snapshot()}
}

// ChooseYtDlpFile apre un selettore file per scegliere l'eseguibile yt-dlp
// personalizzato. Restituisce il percorso scelto (vuoto se annullato). Non
// persiste nulla: la UI applica poi la scelta con SetYtDlpConfig.
func (a *App) ChooseYtDlpFile() string {
	path, err := a.h().ChooseFile("Seleziona l'eseguibile yt-dlp", []FileFilter{
		{DisplayName: "Eseguibili (*.exe)", Pattern: "*.exe"},
		{DisplayName: "Tutti i file (*.*)", Pattern: "*.*"},
	})
	if err != nil {
		return ""
	}
	return path
}

// InstallYtDlp scarica l'ultima versione ufficiale di yt-dlp nel percorso
// effettivo in uso (la copia gestita in %AppData%\RenameMusic se "gestisci
// autonomamente" è attivo, altrimenti il percorso personalizzato) sovrascrivendo
// il file eventualmente presente: funge quindi anche da "Aggiorna". Se manca
// ffmpeg (necessario a yt-dlp per l'mp3) scarica anche quello, così un solo
// "Scarica" rende l'app pronta per le playlist.
func (a *App) InstallYtDlp() ActionResponse {
	a.mu.Lock()
	dest := a.ytDlpEffectivePath()
	a.mu.Unlock()

	if dest == "" {
		msg := "Specifica un percorso per yt-dlp o attiva la gestione automatica."
		return ActionResponse{OK: false, Message: msg, State: a.snapshotLocked()}
	}

	a.mu.Lock()
	a.addLogLocked(LogInfo, "Download di yt-dlp in corso...")
	a.mu.Unlock()

	a.ytDlpMu.Lock()
	err := a.yt().Install(dest, a.installProgress("yt-dlp"))
	a.ytDlpMu.Unlock()

	a.mu.Lock()
	a.refreshYtDlpStatus()
	if err != nil {
		defer a.mu.Unlock()
		a.addLogLocked(LogError, "Installazione di yt-dlp fallita: "+err.Error())
		return ActionResponse{OK: false, Message: "Installazione di yt-dlp fallita: " + err.Error(), State: a.snapshot()}
	}
	msg := "yt-dlp installato."
	if a.ytDlpVersion != "" {
		msg = "yt-dlp installato (versione " + a.ytDlpVersion + ")."
	}
	a.addLogLocked(LogSuccess, msg)
	needFFmpeg := !a.ffmpegAvailable
	a.mu.Unlock()

	if needFFmpeg {
		return a.InstallFFmpeg()
	}
	return ActionResponse{OK: true, Message: msg, State: a.snapshotLocked()}
}

// InstallFFmpeg scarica l'ultima build di ffmpeg nella copia gestita dall'app
// (%AppData%\RenameMusic\ffmpeg), usata da yt-dlp per estrarre l'audio in mp3.
// Funziona sia con yt-dlp gestito sia con quello scelto a mano.
func (a *App) InstallFFmpeg() ActionResponse {
	a.mu.Lock()
	a.addLogLocked(LogInfo, "Download di ffmpeg in corso (circa 200 MB)...")
	a.mu.Unlock()

	err := a.yt().InstallFFmpeg(a.installProgress("ffmpeg"))

	a.mu.Lock()
	defer a.mu.Unlock()
	a.refreshYtDlpStatus()
	if err != nil {
		a.addLogLocked(LogError, "Installazione di ffmpeg fallita: "+err.Error())
		return ActionResponse{OK: false, Message: "Installazione di ffmpeg fallita: " + err.Error(), State: a.snapshot()}
	}
	a.addLogLocked(LogSuccess, "ffmpeg installato.")
	return ActionResponse{OK: true, Message: "ffmpeg installato.", State: a.snapshot()}
}

// UninstallYtDlp rimuove la copia di yt-dlp gestita dall'app
// (%AppData%\RenameMusic). Ha effetto solo in gestione automatica: in modalità
// manuale il file è scelto dall'utente e non deve essere cancellato dall'app.
func (a *App) UninstallYtDlp() ActionResponse {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.ytDlpManaged {
		msg := "In modalità manuale yt-dlp non viene rimosso: il file è tuo."
		return ActionResponse{OK: false, Message: msg, State: a.snapshot()}
	}

	path, err := a.yt().ManagedPath()
	if err != nil {
		msg := "Percorso di yt-dlp non determinabile: " + err.Error()
		a.addLogLocked(LogError, msg)
		return ActionResponse{OK: false, Message: msg, State: a.snapshot()}
	}

	if err := a.yt().Uninstall(path); err != nil {
		msg := "Rimozione di yt-dlp fallita: " + err.Error()
		a.addLogLocked(LogError, msg)
		return ActionResponse{OK: false, Message: msg, State: a.snapshot()}
	}

	a.refreshYtDlpStatus()
	a.addLogLocked(LogSuccess, "yt-dlp rimosso.")
	return ActionResponse{OK: true, Message: "yt-dlp rimosso.", State: a.snapshot()}
}

// DownloadPlaylist scarica in mp3 (in una cartella già selezionata come
// cartella di partenza) tutti i video della playlist YouTube associata al
// nome `name`, poi esegue una scansione così l'anteprima si aggiorna con i
// nuovi file. Richiede yt-dlp disponibile (vedi YtDlp).
func (a *App) DownloadPlaylist(name string) ActionResponse {
	opCtx, endOp := a.beginCancelable()
	defer endOp()
	resp, _ := a.downloadPlaylist(opCtx, name)
	return resp
}

// DownloadAndProcess è l'azione della modalità semplificata: scarica la
// playlist `name` come DownloadPlaylist e poi converte subito tutti i file della
// cartella come ProcessAll, senza passare dall'anteprima. È un'unica operazione
// annullabile: se l'annullamento arriva durante il download, la conversione non
// parte. La risposta è quella della conversione (risultati e tracce da
// confermare) con in più i video non scaricati; se il download non va a buon
// fine (o viene annullato) è quella del download.
func (a *App) DownloadAndProcess(name string) ActionResponse {
	// La destinazione si valida prima di scaricare: scoprire solo a download
	// finito che la conversione non può partire sarebbe una perdita di tempo.
	a.mu.Lock()
	destSame, destFolder := a.destSameAsSource, a.destFolder
	a.mu.Unlock()
	if _, msg := conversionDestination(destSame, destFolder); msg != "" {
		return ActionResponse{OK: false, Message: msg, State: a.snapshotLocked()}
	}

	opCtx, endOp := a.beginCancelable()
	defer endOp()

	dl, downloaded := a.downloadPlaylist(opCtx, name)
	if !downloaded {
		return dl
	}

	resp := a.processAll(opCtx, nil)
	resp.DownloadErrors = dl.DownloadErrors
	if n := len(dl.DownloadErrors); n > 0 {
		resp.OK = false
		resp.Message = fmt.Sprintf("%s %d download non riusciti.", resp.Message, n)
	}
	return resp
}

// downloadPlaylist esegue il download di DownloadPlaylist (vedi) sotto il
// context dell'operazione in corso. downloaded è true se il download si è
// concluso (anche con errori su singoli video) senza essere annullato e la
// scansione successiva è riuscita: è la condizione perché DownloadAndProcess
// prosegua con la conversione.
func (a *App) downloadPlaylist(opCtx context.Context, name string) (resp ActionResponse, downloaded bool) {
	a.mu.Lock()
	folder := a.config.StartFolder
	ytdlp := a.ytDlpEffectivePath()
	var url string
	found := false
	for _, p := range a.playlists {
		if p.Name == name {
			url = p.URL
			found = true
			break
		}
	}
	a.mu.Unlock()

	if !found {
		return ActionResponse{OK: false, Message: "Playlist non trovata.", State: a.snapshotLocked()}, false
	}
	// Anche la destinazione: dopo il download la scansione (e, in modalità
	// semplificata, la conversione) la richiede, meglio non scaricare invano.
	if msg := a.foldersError(); msg != "" {
		return ActionResponse{OK: false, Message: msg, State: a.snapshotLocked()}, false
	}

	if !a.yt().Available(ytdlp) {
		return ActionResponse{OK: false, Message: "yt-dlp non disponibile: scaricalo o imposta un percorso valido nelle impostazioni.", State: a.snapshotLocked()}, false
	}
	if !a.yt().FFmpegAvailable() {
		return ActionResponse{OK: false, Message: "ffmpeg non disponibile: scaricalo dalle impostazioni (serve a yt-dlp per creare gli mp3).", State: a.snapshotLocked()}, false
	}

	a.mu.Lock()
	// Sospende l'aggiornamento automatico per la durata del download: i file
	// scaricati generano una raffica di eventi fsnotify che altrimenti farebbero
	// ripartire scansioni a metà download. Il watcher viene riabilitato dalla
	// performScan finale (che azzera watchPaused) una volta scaricato tutto.
	a.watchPaused = true
	a.addLogLocked(LogInfo, fmt.Sprintf("Avvio download playlist %q...", name))
	a.mu.Unlock()

	// Se l'aggiornamento automatico sta sostituendo yt-dlp, si parte appena ha
	// finito (pochi secondi), con la versione nuova.
	if !a.ytDlpMu.TryLock() {
		a.mu.Lock()
		a.addLogLocked(LogInfo, "Aggiornamento di yt-dlp in corso: il download parte appena termina...")
		a.mu.Unlock()
		a.ytDlpMu.Lock()
	}
	result, err := playlist.Download(playlist.Options{
		Runner:  a.yt().Runner(ytdlp),
		URL:     url,
		Folder:  folder,
		Workers: a.yt().Workers(),
		OnProgress: func(done, total int) {
			a.emit(EventProcessProgress, ProgressEvent{Done: done, Total: total, Phase: PhaseDownload})
		},
		Cancelled: func() bool { return opCtx.Err() != nil },
	})
	a.ytDlpMu.Unlock()
	canceled := opCtx.Err() != nil

	if err != nil {
		a.mu.Lock()
		a.watchPaused = false // download fallito senza rescan: riabilita l'aggiornamento automatico
		a.addLogLocked(LogError, "Download playlist fallito: "+err.Error())
		state := a.snapshot()
		a.mu.Unlock()
		return ActionResponse{OK: false, Message: "Download fallito: " + err.Error(), State: state}, false
	}

	a.mu.Lock()
	switch {
	case canceled:
		a.addLogLocked(LogInfo, fmt.Sprintf("Download annullato: %d completati, %d falliti.", result.Downloaded, result.Failed))
	case result.Failed > 0:
		a.addLogLocked(LogError, fmt.Sprintf("Playlist %q: %d file scaricati, %d falliti.", name, result.Downloaded, result.Failed))
	default:
		a.addLogLocked(LogSuccess, fmt.Sprintf("Playlist %q scaricata: %d file.", name, result.Downloaded))
	}
	a.mu.Unlock()

	// Scansiona la cartella per aggiornare l'anteprima con i nuovi file.
	message, ok := a.performScan()
	finalState := a.snapshotLocked()

	// Dettaglio dei video non scaricati, per il modale in UI (vuoto => omesso).
	dlErrors := make([]DownloadErrorView, 0, len(result.Failures))
	for _, f := range result.Failures {
		dlErrors = append(dlErrors, DownloadErrorView{
			VideoID: f.VideoID,
			Title:   f.Title,
			URL:     f.URL,
			Message: f.Message,
		})
	}

	if canceled {
		return ActionResponse{OK: ok, Message: "Download annullato. " + message, State: finalState, DownloadErrors: dlErrors}, false
	}
	if result.Failed > 0 {
		return ActionResponse{OK: false, Message: fmt.Sprintf("Download completato con %d errori.", result.Failed), State: finalState, DownloadErrors: dlErrors}, ok
	}
	return ActionResponse{OK: ok, Message: message, State: finalState}, ok
}

// ChooseDirectory apre il selettore cartella e restituisce il percorso scelto
// (stringa vuota se annullato). Usato per selezionare la cartella di destinazione.
func (a *App) ChooseDirectory() string {
	path, err := a.h().ChooseDirectory("Seleziona cartella di destinazione")
	if err != nil {
		return ""
	}
	return path
}

// beginCancelable prepara un'operazione lunga cancellabile: crea un context
// annullabile, lo registra come operazione corrente (per Cancel) e restituisce
// il context più una funzione di cleanup da invocare in defer.
func (a *App) beginCancelable() (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	if a.opCancel != nil {
		a.opCancel() // difensivo: chiude un'eventuale operazione precedente non ripulita
	}
	a.opCancel = cancel
	a.mu.Unlock()

	return ctx, func() {
		a.mu.Lock()
		a.opCancel = nil
		a.mu.Unlock()
		cancel()
	}
}

// Cancel richiede l'annullamento dell'operazione lunga in corso (ProcessAll o
// ClearTags). Se non ce n'è alcuna, non fa nulla.
func (a *App) Cancel() ActionResponse {
	a.mu.Lock()
	cancel := a.opCancel
	if cancel != nil {
		a.addLogLocked(LogInfo, "Annullamento richiesto…")
	}
	state := a.snapshot()
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return ActionResponse{OK: true, State: state}
}

// ProcessAll esegue in un colpo solo normalizzazione dei nomi + scrittura tag.
// destination vuota => stessa cartella di partenza. deleteOriginals=false scrive
// una copia lasciando intatti gli originali (e gli altri file presenti).
// review sono i percorsi che l'utente ha selezionato nell'anteprima per
// rivederli: non vengono convertiti subito ma tornano alla UI come tracce da
// confermare, come quelle con tag sconosciuti.
func (a *App) ProcessAll(review []string) ActionResponse {
	opCtx, endOp := a.beginCancelable()
	defer endOp()
	return a.processAll(opCtx, review)
}

// conversionDestination restituisce la cartella di destinazione della
// conversione ("" = la cartella di partenza) o, se quella scelta non è
// utilizzabile, il messaggio d'errore da mostrare. Fa I/O su disco: va chiamata
// SENZA lock.
func conversionDestination(sameAsSource bool, folder string) (destination, errMsg string) {
	if sameAsSource {
		return "", ""
	}
	if folder == "" {
		return "", "Scegli una cartella di destinazione."
	}
	if !appfs.IsDir(folder) {
		return "", "Cartella di destinazione non trovata: " + folder
	}
	return folder, ""
}

// processAll esegue la conversione di ProcessAll (vedi) sotto il context
// dell'operazione in corso, così DownloadAndProcess la esegue nella stessa
// operazione annullabile del download. review: vedi ProcessAll.
func (a *App) processAll(opCtx context.Context, review []string) ActionResponse {
	a.mu.Lock()
	cfg := a.config
	destSame := a.destSameAsSource
	destFolder := a.destFolder
	deleteOriginals := a.deleteOriginals
	files := append([]string(nil), a.scanned...)
	a.mu.Unlock()

	if msg := a.foldersError(); msg != "" {
		return ActionResponse{OK: false, Message: msg, State: a.snapshotLocked()}
	}
	destination, destErr := conversionDestination(destSame, destFolder)
	if destErr != "" {
		return ActionResponse{OK: false, Message: destErr, State: a.snapshotLocked()}
	}

	// Rimuove i temporanei orfani da conversioni precedenti interrotte, in
	// origine e (se distinta) in destinazione, prima di riscrivere.
	cleaned := rename.CleanTempFiles(cfg.StartFolder)
	if destination != "" && destination != cfg.StartFolder {
		cleaned += rename.CleanTempFiles(destination)
	}
	if cleaned > 0 {
		a.mu.Lock()
		a.addLogLocked(LogInfo, fmt.Sprintf("Rimossi %d file temporanei residui.", cleaned))
		a.mu.Unlock()
	}

	service := rename.NewService(cfg)
	if files == nil {
		var err error
		files, err = service.Scan()
		if err != nil {
			return ActionResponse{OK: false, Message: "Errore scansione: " + err.Error(), State: a.snapshotLocked()}
		}
	}

	// Separa le tracce che, una volta normalizzate, avrebbero tag sconosciuti
	// (titolo o artista) da quelle già a posto. Le seconde le convertiamo subito
	// qui; per le prime NON blocchiamo nulla: le restituiamo alla UI come
	// `prompts`, che chiederà all'utente come procedere (una alla volta) e
	// risolverà ognuna con una chiamata a sé (ResolveTagPrompt). Allo stesso
	// modo le tracce selezionate dall'utente per essere riviste; una traccia
	// selezionata ma con tag sconosciuti resta segnalata come tale.
	toReview := make(map[string]bool, len(review))
	for _, p := range review {
		toReview[p] = true
	}
	var goodFiles []string
	var prompts []TagPromptView
	unknownCount, reviewCount := 0, 0
	for _, p := range files {
		title, artist, unknown := unknownTagFor(p, cfg)
		if !unknown && !toReview[p] {
			goodFiles = append(goodFiles, p)
			continue
		}
		name := filepath.Base(p)
		prompt := TagPromptView{
			Path:         p,
			OriginalBase: parser.RemoveExtension(name),
			Ext:          parser.Extension(name),
			Title:        title,
			Artist:       artist,
		}
		if unknown {
			unknownCount++
		} else {
			reviewCount++
			prompt.Review = true
			prompt.PreviewBase = cfg.NormalizeFileBase(prompt.OriginalBase)
		}
		prompts = append(prompts, prompt)
	}

	total := len(goodFiles)
	done := 0
	// Totale subito (0 completati), come per il download: dopo un
	// DownloadAndProcess la barra passa così alla conversione senza restare
	// ferma sull'ultimo avanzamento del download fino al primo file convertito.
	if total > 0 {
		a.emit(EventProcessProgress, ProgressEvent{Done: 0, Total: total, Phase: PhaseConvert})
	}
	results, _ := service.Process(goodFiles, rename.Options{
		DestinationFolder: destination,
		DeleteOriginals:   deleteOriginals,
		OnProgress: func(_, _ int) {
			done++
			a.emit(EventProcessProgress, ProgressEvent{Done: done, Total: total, Phase: PhaseConvert})
		},
		Cancelled: func() bool { return opCtx.Err() != nil },
	})
	canceled := opCtx.Err() != nil

	tagged, skipped, failed, canceledFiles := 0, 0, 0, 0
	views := make([]ResultView, 0, len(results))
	for _, result := range results {
		// Failed è esclusivo nel conteggio: un file fallito non è né "elaborato"
		// né "saltato con successo", anche se collideva (Skipped+Failed insieme).
		switch {
		case result.Canceled:
			canceledFiles++
		case result.Failed:
			failed++
		case result.Skipped:
			skipped++
		default:
			if result.Tagged {
				tagged++
			}
		}
		views = append(views, resultViewFor(result, cfg))
	}
	processed := len(results) - skipped - failed - canceledFiles

	a.mu.Lock()
	a.scanned = nil
	a.currentTags = nil
	// Da qui in avanti l'UI mostra i risultati (poi "Avvia nuova scansione"):
	// ignoriamo gli eventi del watcher (compresi quelli auto-generati da questa
	// elaborazione, e da quella delle tracce da confermare) fino alla prossima
	// Scan/cambio cartella.
	a.watchPaused = true
	if canceled {
		a.addLogLocked(LogInfo, fmt.Sprintf("Elaborazione annullata: %d file elaborati (%d con tag MP3).", processed, tagged))
	} else {
		a.addLogLocked(LogSuccess, fmt.Sprintf("Elaborati %d file (%d con tag MP3).", processed, tagged))
	}
	if skipped > 0 {
		if deleteOriginals {
			a.addLogLocked(LogInfo, fmt.Sprintf("Saltati ed eliminati %d file.", skipped))
		} else {
			a.addLogLocked(LogInfo, fmt.Sprintf("Saltati %d file.", skipped))
		}
	}
	if failed > 0 {
		a.addLogLocked(LogError, fmt.Sprintf("%d file non elaborati per errori (dettagli nella tabella).", failed))
	}
	if unknownCount > 0 && !canceled {
		a.addLogLocked(LogInfo, fmt.Sprintf("%d tracce senza titolo/artista: in attesa di conferma.", unknownCount))
	}
	if reviewCount > 0 && !canceled {
		a.addLogLocked(LogInfo, fmt.Sprintf("%d tracce selezionate da rivedere: in attesa di conferma.", reviewCount))
	}
	state := a.snapshot()
	a.mu.Unlock()

	if canceled {
		return ActionResponse{
			OK:      failed == 0,
			Message: fmt.Sprintf("Elaborazione annullata: %d file elaborati.", processed),
			State:   state,
			Results: views,
			Prompts: prompts,
		}
	}
	if failed > 0 {
		return ActionResponse{
			OK:      false,
			Message: fmt.Sprintf("Elaborazione completata con %d errori.", failed),
			State:   state,
			Results: views,
			Prompts: prompts,
		}
	}
	msg := "Elaborazione completata."
	if len(prompts) > 0 {
		msg = fmt.Sprintf("Elaborati %d file; %d tracce richiedono una scelta.", processed, len(prompts))
	}
	return ActionResponse{OK: true, Message: msg, State: state, Results: views, Prompts: prompts}
}

// resultViewFor costruisce la ResultView (per la tabella dei risultati) da un
// esito di conversione, ricavando i tag scritti dal nuovo nome come
// nell'anteprima. Condivisa da ProcessAll e ResolveTagPrompt.
func resultViewFor(result rename.Result, cfg rules.Config) ResultView {
	view := ResultView{
		OldName:  filepath.Base(result.OldPath),
		NewName:  result.NewName,
		Tagged:   result.Tagged,
		Skipped:  result.Skipped,
		Failed:   result.Failed,
		Canceled: result.Canceled,
		Reason:   result.Reason,
	}
	if parser.Extension(result.NewName) == "mp3" {
		view.MP3 = true
		view.Title = parser.TagTitle(result.NewName, cfg.FtDestination(), cfg.ArtistExceptions)
		view.Artist = parser.TagArtist(result.NewName, cfg.FtDestination(), cfg.ArtistExceptions)
	}
	return view
}

// unknownTagFor calcola i tag che verrebbero scritti per `path` a partire dal
// nome normalizzato (nessun I/O) e indica se titolo o artista risulterebbero
// "sconosciuti". Solo gli MP3 producono tag: per gli altri isUnknown è false.
func unknownTagFor(path string, cfg rules.Config) (title, artist string, isUnknown bool) {
	name := filepath.Base(path)
	ext := parser.Extension(name)
	if ext != "mp3" {
		return "", "", false
	}
	preview := cfg.NormalizeFileBase(parser.RemoveExtension(name)) + "." + ext
	title = parser.TagTitle(preview, cfg.FtDestination(), cfg.ArtistExceptions)
	artist = parser.TagArtist(preview, cfg.FtDestination(), cfg.ArtistExceptions)
	return title, artist, title == parser.UnknownTitle || artist == parser.UnknownArtist
}

// ResolveTagPrompt converte una singola traccia segnalata da ProcessAll come
// avente tag sconosciuti, secondo la scelta dell'utente: useEdited=false
// ("Salta") la converte col nome originale (i tag resteranno "sconosciuto");
// useEdited=true ("Continua") usa editedBase come nuovo nome — normalizzato
// dalle regole correnti e con i tag riestratti da esso. Restituisce l'esito
// come singolo Result, che la UI aggiunge alla tabella dei risultati. Usa le
// stesse opzioni (destinazione/eliminazione) persistite, già validate da
// ProcessAll; non si blocca né attende nulla.
func (a *App) ResolveTagPrompt(path string, useEdited bool, editedBase string) ActionResponse {
	a.mu.Lock()
	cfg := a.config
	destSame := a.destSameAsSource
	destFolder := a.destFolder
	deleteOriginals := a.deleteOriginals
	a.mu.Unlock()

	destination := ""
	if !destSame {
		destination = destFolder
	}

	opts := rename.Options{DestinationFolder: destination, DeleteOriginals: deleteOriginals}
	if useEdited {
		if b := strings.TrimSpace(editedBase); b != "" {
			opts.NameOverrides = map[string]string{path: b}
		}
	}

	results, _ := rename.NewService(cfg).Process([]string{path}, opts)

	views := make([]ResultView, 0, len(results))
	for _, r := range results {
		views = append(views, resultViewFor(r, cfg))
	}

	origBase := parser.RemoveExtension(filepath.Base(path))
	a.mu.Lock()
	switch {
	case len(results) == 0:
		// Percorso non supportato/estensione ignota: nessun esito da riportare.
	case results[0].Failed:
		a.addLogLocked(LogError, fmt.Sprintf("Traccia %q non elaborata: %s", origBase, results[0].Reason))
	case results[0].Skipped:
		a.addLogLocked(LogInfo, fmt.Sprintf("Traccia %q saltata: %s", origBase, results[0].Reason))
	case useEdited:
		a.addLogLocked(LogSuccess, fmt.Sprintf("Traccia rinominata in %q.", parser.RemoveExtension(results[0].NewName)))
	default:
		a.addLogLocked(LogInfo, fmt.Sprintf("Traccia %q convertita col nome originale.", origBase))
	}
	state := a.snapshot()
	a.mu.Unlock()

	ok := len(results) > 0 && !results[0].Failed
	return ActionResponse{OK: ok, State: state, Results: views}
}

// ClearTags cancella TUTTI i tag ID3 dagli MP3 attualmente scansionati, in posto
// (senza rinominare né spostare). Azione distruttiva: la UI la conferma prima.
func (a *App) ClearTags() ActionResponse {
	a.mu.Lock()
	cfg := a.config
	files := append([]string(nil), a.scanned...)
	a.mu.Unlock()

	if len(files) == 0 {
		return ActionResponse{OK: false, Message: "Nessun file da elaborare.", State: a.snapshotLocked()}
	}

	opCtx, endOp := a.beginCancelable()
	defer endOp()

	cleared, failed := rename.NewService(cfg).ClearTags(files,
		func(done, total int) {
			a.emit(EventProcessProgress, ProgressEvent{Done: done, Total: total})
		},
		func() bool { return opCtx.Err() != nil },
	)
	canceled := opCtx.Err() != nil

	// I tag su disco sono cambiati per gli stessi percorsi scansionati: rilegge
	// i tag attuali (i file appena ripuliti risulteranno senza titolo/artista).
	currentTags := currentTagsFor(files)

	a.mu.Lock()
	a.currentTags = currentTags
	if canceled {
		a.addLogLocked(LogInfo, fmt.Sprintf("Cancellazione tag annullata: %d file ripuliti.", cleared))
	} else {
		a.addLogLocked(LogSuccess, fmt.Sprintf("Cancellati i tag di %d file MP3.", cleared))
	}
	if failed > 0 {
		a.addLogLocked(LogError, fmt.Sprintf("Cancellazione tag fallita per %d file.", failed))
	}
	state := a.snapshot()
	a.mu.Unlock()

	if canceled {
		return ActionResponse{OK: failed == 0, Message: fmt.Sprintf("Cancellazione annullata: tag rimossi da %d file.", cleared), State: state}
	}
	if failed > 0 {
		return ActionResponse{OK: false, Message: fmt.Sprintf("Tag cancellati per %d file, %d falliti.", cleared, failed), State: state}
	}
	return ActionResponse{OK: true, Message: fmt.Sprintf("Tag cancellati per %d file.", cleared), State: state}
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
		cfg := a.currentConfig()
		if files, err := rename.NewService(cfg).Scan(); err == nil {
			currentTags := currentTagsFor(files)
			a.mu.Lock()
			a.scanned = files
			a.currentTags = currentTags
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

// watchLiveLocked indica se gli eventi del watcher vanno seguiti: aggiornamento
// automatico attivo e non in pausa, e fuori dalla modalità semplificata (che non
// ha un'anteprima da aggiornare). Va chiamata con il lock acquisito.
func (a *App) watchLiveLocked() bool {
	return a.watcher != nil && a.watchEnabled && !a.watchPaused && !a.config.SimpleMode
}

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
	if !a.watchLiveLocked() {
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
	if !a.watchLiveLocked() {
		a.mu.Unlock()
		return
	}
	cfg := a.config
	a.mu.Unlock()

	if !appfs.IsDir(cfg.StartFolder) {
		// La cartella osservata è sparita: svuotiamo l'anteprima e lo
		// segnaliamo subito alla UI (che mostra l'avviso).
		a.mu.Lock()
		a.scanned = nil
		a.currentTags = nil
		a.addLogLocked(LogError, "Cartella di partenza non trovata: "+cfg.StartFolder)
		state := a.snapshot()
		a.mu.Unlock()
		a.emit(EventWatchChanged, state)
		return
	}

	files, err := rename.NewService(cfg).Scan()
	if err != nil {
		a.mu.Lock()
		a.addLogLocked(LogError, "Aggiornamento automatico: errore scansione: "+err.Error())
		a.mu.Unlock()
		return
	}
	currentTags := currentTagsFor(files)

	a.mu.Lock()
	a.scanned = files
	a.currentTags = currentTags
	a.addLogLocked(LogAuto, fmt.Sprintf("Scansione automatica: %d file audio.", len(files)))
	state := a.snapshot()
	a.mu.Unlock()

	a.emit(EventWatchChanged, state)
}

func (a *App) onWatchError(err error) {
	a.mu.Lock()
	a.addLogLocked(LogError, "Aggiornamento automatico: errore filesystem: "+err.Error())
	a.mu.Unlock()
}

// rescanWith esegue una scansione con la config fornita. Fa I/O su disco, quindi
// va chiamata SENZA lock. Se la cartella non è valida ritorna rescanned=false
// (nessuna scansione, nessun errore); se la scansione fallisce ritorna l'errore
// e rescanned=false, così il chiamante lascia invariato lo stato precedente.
func (a *App) rescanWith(cfg rules.Config) (files []string, currentTags map[string]rename.TagInfo, rescanned bool, err error) {
	if !appfs.IsDir(cfg.StartFolder) {
		return nil, nil, false, nil
	}
	files, err = rename.NewService(cfg).Scan()
	if err != nil {
		return nil, nil, false, err
	}
	currentTags = currentTagsFor(files)
	return files, currentTags, true, nil
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
		view := FileView{
			Name:    name,
			Path:    path,
			Preview: preview,
			MP3:     ext == "mp3",
		}
		if view.MP3 {
			current := a.currentTags[path]
			view.Title = current.Title
			view.Artist = current.Artist
			view.TitlePreview = parser.TagTitle(preview, a.config.FtDestination(), a.config.ArtistExceptions)
			view.ArtistPreview = parser.TagArtist(preview, a.config.FtDestination(), a.config.ArtistExceptions)
		}
		files = append(files, view)
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
		// Solo uno stat per cartella: abbastanza leggero da farlo a ogni snapshot.
		FolderMissing:      a.config.StartFolder != "" && !appfs.IsDir(a.config.StartFolder),
		DestinationMissing: !a.destSameAsSource && a.destFolder != "" && !appfs.IsDir(a.destFolder),
		WatchEnabled:       a.watchEnabled,
		WatchActive:        a.watcher != nil && a.watcher.Folder() != "",
		Playlists:          append([]playlist.Playlist(nil), a.playlists...),
		YtDlpManaged:       a.ytDlpManaged,
		YtDlpPath:          a.ytDlpPath,
		YtDlpEffectivePath: a.ytDlpEffectivePath(),
		YtDlpAvailable:     a.ytDlpAvailable,
		YtDlpVersion:       a.ytDlpVersion,
		FFmpegAvailable:    a.ffmpegAvailable,
		AppVersion:         update.Version,
		Update:             a.updateViewLocked(),
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
	cfg.FtAlias = strings.TrimSpace(cfg.FtAlias)
	cfg.ArtistExceptions = cleanList(cfg.ArtistExceptions)

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
