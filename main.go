package main

import (
	"context"
	"embed"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"renamemusic/internal/core"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Istanza del core applicativo (internal/core), condiviso con l'app Android:
	// qui gli si forniscono i servizi di piattaforma desktop (Wails + yt-dlp.exe
	// + aggiornamento dell'eseguibile).
	host := &desktopHost{}
	app := core.New(core.Options{Host: host, YtDlp: core.ExecYtDlp{}, Updater: desktopUpdater{host: host}})

	// Configurazione Wails
	err := wails.Run(&options.App{
		Title:  "RenameMusic",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			// Frontend React buildato in frontend/dist ed embeddato nell'eseguibile.
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 246, G: 247, B: 249, A: 1},
		OnStartup: func(ctx context.Context) {
			host.ctx = ctx
			core.Start(app)
			// Trascinamento di una cartella (o file) sulla finestra: imposta la
			// cartella di partenza. Avviene fuori dal ciclo richiesta/risposta
			// della UI: il core notifica il frontend con un evento dedicato.
			wailsruntime.OnFileDrop(ctx, func(_, _ int, paths []string) {
				core.HandleFileDrop(app, paths)
			})
		},
		// Abilita il trascinamento di file/cartelle sulla finestra: il percorso
		// rilasciato viene gestito in OnStartup via runtime.OnFileDrop.
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true,
		},
		Bind: []interface{}{
			app,
		},
		// L'icona dell'app su Windows viene presa da build/windows/icon.ico durante `wails build`.
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Errore avvio Wails:", err)
		os.Exit(1)
	}
}
