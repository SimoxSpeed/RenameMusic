package menu

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	appfs "renamemusic/internal/fs"
	"renamemusic/internal/rename"
	"renamemusic/internal/rules"
)

func Run() {
	reader := bufio.NewReader(os.Stdin)
	folder := rules.DefaultStartFolder
	service := rename.NewService(folder)
	var scanned []string

	for {
		fmt.Println()
		fmt.Println("Rename Music")
		fmt.Println("Cartella:", folder)
		fmt.Println("1. Seleziona cartella")
		fmt.Println("2. Avvia scansione")
		fmt.Println("3. Rinomina file")
		fmt.Println("4. Scrivi tag")
		fmt.Println("5. Esci")
		fmt.Print("Scelta: ")

		choice := readLine(reader)
		switch choice {
		case "1":
			fmt.Print("Percorso cartella: ")
			selected := strings.Trim(readLine(reader), `"`)
			if !appfs.IsDir(selected) {
				fmt.Println("Cartella non valida.")
				continue
			}
			folder = selected
			service = rename.NewService(folder)
			scanned = nil
			fmt.Println("Cartella selezionata:", folder)
		case "2":
			files, err := service.Scan()
			if err != nil {
				fmt.Println("Errore scansione:", err)
				continue
			}
			scanned = files
			printScanned(scanned)
		case "3":
			files, err := ensureScanned(service, scanned)
			if err != nil {
				fmt.Println("Errore scansione:", err)
				continue
			}
			results, err := service.RenameAll(files)
			if err != nil {
				fmt.Println("Errore rinomina:", err)
				continue
			}
			scanned = nil
			for i, result := range results {
				tagText := ""
				if result.Tagged {
					tagText = " [tag MP3]"
				}
				fmt.Printf("--- %d/%d ---%s\n%s\n", i+1, len(results), tagText, result.NewName)
			}
			fmt.Println("------------------------------------ TERMINATO ------------------------------------")
		case "4":
			files, err := ensureScanned(service, scanned)
			if err != nil {
				fmt.Println("Errore scansione:", err)
				continue
			}
			written, err := rename.WriteTagsAll(files)
			if err != nil {
				fmt.Println("Errore scrittura tag:", err)
				continue
			}
			fmt.Printf("Tag scritti su %d file MP3.\n", written)
		case "5":
			fmt.Println("Uscita.")
			return
		default:
			fmt.Println("Scelta non valida.")
		}
	}
}

func ensureScanned(service *rename.Service, scanned []string) ([]string, error) {
	if scanned != nil {
		return scanned, nil
	}
	return service.Scan()
}

func printScanned(files []string) {
	if len(files) == 0 {
		fmt.Println("Nessun file audio supportato trovato.")
		return
	}
	for i, path := range files {
		fmt.Printf("%d. %s\n", i+1, filepath.Base(path))
	}
	fmt.Printf("Totale file audio: %d\n", len(files))
}

func readLine(reader *bufio.Reader) string {
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}
