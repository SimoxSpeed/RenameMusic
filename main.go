package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"renamemusic/internal/menu"
)

func main() {
	cli := flag.Bool("cli", false, "avvia il menu testuale")
	flag.Parse()

	if *cli {
		menu.Run()
		return
	}

	// Istanza dell'app (definita in app.go)
	app := NewApp()

	// Configurazione Wails
	err := wails.Run(&options.App{
		Title:  "RenameMusic",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assetserver.BundleAssets{
				// I file frontend si trovano in frontend/dist
				// Wails li embedderà nell'eseguibile
				Path: "frontend/dist",
			},
		},
		BackgroundColour: &options.RGBA{R: 246, G: 247, B: 249, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
		Windows: &options.Windows{
			Icon: "Icone/Rename Music Icon.ico",
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Errore avvio Wails:", err)
		os.Exit(1)
	}
}