package main

import (
	"context"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"renamemusic/internal/core"
)

// desktopHost implementa core.Host sopra il runtime Wails: eventi verso la UI,
// dialog nativi di sistema e apertura della cartella in Esplora risorse. ctx è
// il context Wails ricevuto in OnStartup, valorizzato prima di qualsiasi
// chiamata della UI.
type desktopHost struct {
	ctx context.Context
}

func (h *desktopHost) Emit(event string, payload any) {
	if h.ctx != nil {
		wailsruntime.EventsEmit(h.ctx, event, payload)
	}
}

func (h *desktopHost) ChooseDirectory(title string) (string, error) {
	return wailsruntime.OpenDirectoryDialog(h.ctx, wailsruntime.OpenDialogOptions{Title: title})
}

func (h *desktopHost) ChooseFile(title string, filters []core.FileFilter) (string, error) {
	wf := make([]wailsruntime.FileFilter, 0, len(filters))
	for _, f := range filters {
		wf = append(wf, wailsruntime.FileFilter{DisplayName: f.DisplayName, Pattern: f.Pattern})
	}
	return wailsruntime.OpenFileDialog(h.ctx, wailsruntime.OpenDialogOptions{Title: title, Filters: wf})
}

func (h *desktopHost) OpenFolder(path string) error {
	return openFolderInShell(path)
}
