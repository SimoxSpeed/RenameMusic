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
)

type App struct {
	ctx     context.Context
	mu      sync.Mutex
	folder  string
	scanned []string
	logs    []string
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
}

type StateResponse struct {
	Folder string     `json:"folder"`
	Files  []FileView `json:"files"`
	Logs   []string   `json:"logs"`
}

type ActionResponse struct {
	OK      bool          `json:"ok"`
	Message string        `json:"message"`
	State   StateResponse `json:"state"`
	Results []ResultView  `json:"results,omitempty"`
}

func NewApp() *App {
	return &App{
		folder: rules.DefaultStartFolder,
		logs:   []string{"App pronta."},
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) GetState() ActionResponse {
	return ActionResponse{OK: true, State: a.snapshotLocked()}
}

func (a *App) SetFolder(path string) ActionResponse {
	path = strings.Trim(path, `" `)
	if !appfs.IsDir(path) {
		return ActionResponse{OK: false, Message: "La cartella indicata non esiste.", State: a.snapshotLocked()}
	}

	a.mu.Lock()
	a.folder = path
	a.scanned = nil
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

func (a *App) Scan() ActionResponse {
	a.mu.Lock()
	folder := a.folder
	a.mu.Unlock()

	files, err := rename.NewService(folder).Scan()
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

func (a *App) RenameFiles() ActionResponse {
	a.mu.Lock()
	folder := a.folder
	files := append([]string(nil), a.scanned...)
	a.mu.Unlock()

	service := rename.NewService(folder)
	if files == nil {
		var err error
		files, err = service.Scan()
		if err != nil {
			return ActionResponse{OK: false, Message: "Errore scansione: " + err.Error(), State: a.snapshotLocked()}
		}
	}

	results, err := service.RenameAll(files)
	if err != nil {
		return ActionResponse{OK: false, Message: "Errore rinomina: " + err.Error(), State: a.snapshotLocked()}
	}

	views := make([]ResultView, 0, len(results))
	for _, result := range results {
		views = append(views, ResultView{
			OldName: filepath.Base(result.OldPath),
			NewName: result.NewName,
			Tagged:  result.Tagged,
		})
	}

	a.mu.Lock()
	a.scanned = nil
	a.addLogLocked(fmt.Sprintf("Rinomina completata: %d file elaborati.", len(results)))
	state := a.snapshot()
	a.mu.Unlock()

	return ActionResponse{OK: true, Message: "Rinomina completata.", State: state, Results: views}
}

func (a *App) WriteTags() ActionResponse {
	a.mu.Lock()
	folder := a.folder
	files := append([]string(nil), a.scanned...)
	a.mu.Unlock()

	if files == nil {
		var err error
		files, err = rename.NewService(folder).Scan()
		if err != nil {
			return ActionResponse{OK: false, Message: "Errore scansione: " + err.Error(), State: a.snapshotLocked()}
		}
	}

	written, err := rename.WriteTagsAll(files)
	if err != nil {
		return ActionResponse{OK: false, Message: "Errore scrittura tag: " + err.Error(), State: a.snapshotLocked()}
	}

	a.mu.Lock()
	a.addLogLocked(fmt.Sprintf("Tag scritti su %d file MP3.", written))
	state := a.snapshot()
	a.mu.Unlock()

	return ActionResponse{OK: true, Message: "Tag scritti.", State: state}
}

func (a *App) ProcessAll() ActionResponse {
	// 1) Scansione se non già fatta
	a.mu.Lock()
	folder := a.folder
	files := append([]string(nil), a.scanned...)
	a.mu.Unlock()

	if files == nil {
		var err error
		files, err = rename.NewService(folder).Scan()
		if err != nil {
			return ActionResponse{OK: false, Message: "Errore scansione: " + err.Error(), State: a.snapshotLocked()}
		}
		a.mu.Lock()
		a.scanned = files
		a.addLogLocked(fmt.Sprintf("Scansione completata: %d file audio.", len(files)))
		a.mu.Unlock()
	}

	// 2) Rinomina
	service := rename.NewService(folder)
	results, err := service.RenameAll(files)
	if err != nil {
		return ActionResponse{OK: false, Message: "Errore rinomina: " + err.Error(), State: a.snapshotLocked()}
	}

	// dopo rinomina svuota scanned
	a.mu.Lock()
	a.scanned = nil
	a.addLogLocked(fmt.Sprintf("Rinomina completata: %d file elaborati.", len(results)))
	a.mu.Unlock()

	// 3) Scrivi tag
	written, err := rename.WriteTagsAll(results)
	if err != nil {
		return ActionResponse{OK: false, Message: "Errore scrittura tag: " + err.Error(), State: a.snapshotLocked()}
	}

	a.mu.Lock()
	a.addLogLocked(fmt.Sprintf("Tag scritti su %d file MP3.", written))
	a.mu.Unlock()

	return ActionResponse{OK: true, Message: "Tutti i file elaborati e tag aggiornati.", State: a.snapshotLocked()}
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
		preview := rules.NormalizeFileBase(parser.RemoveExtension(name)) + "." + ext
		files = append(files, FileView{
			Name:    name,
			Path:    path,
			Preview: preview,
			MP3:     ext == "mp3",
		})
	}
	logs := append([]string(nil), a.logs...)
	return StateResponse{Folder: a.folder, Files: files, Logs: logs}
}

func (a *App) addLogLocked(message string) {
	a.logs = append([]string{message}, a.logs...)
	if len(a.logs) > 12 {
		a.logs = a.logs[:12]
	}
}
