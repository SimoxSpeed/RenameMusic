package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	appfs "renamemusic/internal/fs"
	"renamemusic/internal/parser"
	"renamemusic/internal/rename"
	"renamemusic/internal/rules"
	"renamemusic/internal/settings"
)

type App struct {
	ctx      context.Context
	mu       sync.Mutex
	config   rules.Config
	defaults rules.Config
	scanned  []string
	logs     []string

	// Opzioni di elaborazione persistite (state.json).
	destSameAsSource bool
	destFolder       string
	deleteOriginals  bool
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

type StateResponse struct {
	Folder                  string       `json:"folder"`
	Files                   []FileView   `json:"files"`
	Logs                    []string     `json:"logs"`
	Config                  rules.Config `json:"config"`
	DestinationSameAsSource bool         `json:"destinationSameAsSource"`
	DestinationFolder       string       `json:"destinationFolder"`
	DeleteOriginals         bool         `json:"deleteOriginals"`
}

type ActionResponse struct {
	OK      bool          `json:"ok"`
	Message string        `json:"message"`
	State   StateResponse `json:"state"`
	Results []ResultView  `json:"results,omitempty"`
}

func NewApp() *App {
	logs := []string{"App pronta."}

	// I default sono persistiti: se il file non esiste lo si crea dal seed di fabbrica.
	defaults, defExisted, err := settings.LoadDefaults()
	if err != nil {
		logs = []string{"Impossibile leggere i default salvati: uso il seed di fabbrica."}
	}
	if !defExisted {
		_ = settings.SaveDefaults(defaults)
	}

	// Le regole correnti: se il file non esiste si inizializzano dai default.
	current, curExisted, err := settings.LoadConfig()
	if err != nil {
		logs = []string{"Impossibile leggere la configurazione salvata: uso i default."}
	}
	if !curExisted {
		current = defaults
		_ = settings.SaveConfig(current)
	} else {
		logs = []string{"Configurazione caricata dal file salvato."}
	}

	// Cartella e opzioni sono persistite a parte: al primo avvio la cartella è vuota.
	st, _ := settings.LoadState()
	current.StartFolder = st.LastFolder
	defaults.StartFolder = st.LastFolder

	return &App{
		config:           current,
		defaults:         defaults,
		logs:             logs,
		destSameAsSource: st.DestinationSameAsSource,
		destFolder:       st.DestinationFolder,
		deleteOriginals:  st.DeleteOriginals,
	}
}

// persistStateLocked salva su disco cartella + opzioni. Va chiamata con il lock acquisito.
func (a *App) persistStateLocked() {
	_ = settings.SaveState(settings.State{
		LastFolder:              a.config.StartFolder,
		DestinationSameAsSource: a.destSameAsSource,
		DestinationFolder:       a.destFolder,
		DeleteOriginals:         a.deleteOriginals,
	})
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) GetState() ActionResponse {
	return ActionResponse{OK: true, State: a.snapshotLocked()}
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
	a.persistStateLocked()
	a.addLogLocked("Cartella selezionata: " + path)
	state := a.snapshot()
	a.mu.Unlock()

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
		a.addLogLocked("Configurazione applicata ma NON salvata: " + saveErr.Error())
	} else {
		a.addLogLocked("Configurazione salvata.")
	}
	state := a.snapshot()
	a.mu.Unlock()

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
		a.addLogLocked("Default aggiornati ma NON salvati su disco: " + defErr.Error())
	} else {
		a.addLogLocked("Nuovo default salvato.")
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
		a.addLogLocked("Default ripristinati ma NON salvati: " + saveErr.Error())
	} else {
		a.addLogLocked("Configurazione ripristinata ai default e salvata.")
	}
	state := a.snapshot()
	a.mu.Unlock()

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
	a.addLogLocked(fmt.Sprintf("Scansione completata: %d file audio.", len(files)))
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
	a.addLogLocked(fmt.Sprintf("Elaborati %d file (%d con tag MP3).", len(results)-skipped, tagged))
	if skipped > 0 {
		if deleteOriginals {
			a.addLogLocked(fmt.Sprintf("Saltati ed eliminati %d file.", skipped))
		} else {
			a.addLogLocked(fmt.Sprintf("Saltati %d file.", skipped))
		}
	}
	state := a.snapshot()
	a.mu.Unlock()

	return ActionResponse{OK: true, Message: "Elaborazione completata.", State: state, Results: views}
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
	logs := append([]string(nil), a.logs...)
	return StateResponse{
		Folder:                  a.config.StartFolder,
		Files:                   files,
		Logs:                    logs,
		Config:                  a.config,
		DestinationSameAsSource: a.destSameAsSource,
		DestinationFolder:       a.destFolder,
		DeleteOriginals:         a.deleteOriginals,
	}
}

func (a *App) addLogLocked(message string) {
	a.logs = append([]string{message}, a.logs...)
	if len(a.logs) > 12 {
		a.logs = a.logs[:12]
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
