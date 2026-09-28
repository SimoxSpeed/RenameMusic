package main

import (
	"os/exec"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"renamemusic/internal/update"
)

// desktopUpdater implementa core.Updater su desktop: la nuova versione viene
// scaricata accanto all'eseguibile, lo sostituisce (vedi
// update.ReplaceExecutable) e l'app si riavvia da sola.
type desktopUpdater struct {
	host *desktopHost
}

// Asset è il nome dell'eseguibile allegato alla release GitHub (lo stesso
// prodotto da `wails build`).
func (desktopUpdater) Asset() string { return "RenameMusic.exe" }

func (desktopUpdater) DownloadPath() (string, error) {
	exe, err := update.Executable()
	if err != nil {
		return "", err
	}
	return exe + update.NewSuffix, nil
}

func (u desktopUpdater) Apply(path string) (string, error) {
	exe, err := update.Executable()
	if err != nil {
		return "", err
	}
	if err := update.ReplaceExecutable(exe, path); err != nil {
		return "", err
	}
	if err := exec.Command(exe).Start(); err != nil {
		return "Nuova versione installata: riapri l'app per usarla.", nil
	}
	// Chiude questa istanza poco dopo, così la risposta arriva alla UI prima
	// che la finestra si chiuda.
	go func() {
		time.Sleep(700 * time.Millisecond)
		wailsruntime.Quit(u.host.ctx)
	}()
	return "Riavvio con la nuova versione…", nil
}

func (desktopUpdater) Cleanup() {
	if exe, err := update.Executable(); err == nil {
		update.CleanupExecutable(exe)
	}
}
