package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Istanza dell'app (definita in app.go)
	app := NewApp()

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
		OnStartup:        app.startup,
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
