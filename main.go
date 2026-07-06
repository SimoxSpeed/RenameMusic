package main

import (
	"flag"
	"fmt"
	"os"

	"renamemusic/internal/menu"
	"renamemusic/internal/webui"
)

func main() {
	cli := flag.Bool("cli", false, "avvia il menu testuale")
	flag.Parse()

	if *cli {
		menu.Run()
		return
	}

	if err := webui.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Errore avvio interfaccia:", err)
		os.Exit(1)
	}
}
