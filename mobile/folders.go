package mobile

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// storageDir è la radice dei volumi di archiviazione su Android: contiene
// "emulated" (memoria interna, utente 0 in emulated/0), "self" e un'entrata per
// ogni scheda SD/USB (es. "1A2B-3C4D").
const storageDir = "/storage"

// internalStorage è la memoria interna condivisa dell'utente principale.
const internalStorage = "/storage/emulated/0"

// FolderRoot è un volume da cui iniziare la navigazione.
type FolderRoot struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// FolderListing è il contenuto di una cartella mostrato dal selettore della UI.
type FolderListing struct {
	Path   string       `json:"path"`
	Parent string       `json:"parent"` // "" se Path è la radice di un volume
	Dirs   []string     `json:"dirs"`   // nomi delle sottocartelle, ordinati
	Roots  []FolderRoot `json:"roots"`
	Error  string       `json:"error,omitempty"`
}

// folderAPI raccoglie i metodi che su Android sostituiscono il selettore
// cartella di sistema: il selettore SAF non restituisce percorsi reali e da
// Android 11 non permette di scegliere né la radice della memoria né Download.
// Con l'accesso a tutti i file (MANAGE_EXTERNAL_STORAGE) navighiamo direttamente
// il filesystem. Sono invocati tramite Call, come i metodi di core.App.
type folderAPI struct{}

// ListDirectory elenca le sottocartelle di path. Con path vuoto parte dalla
// memoria interna; se path non esiste risale alla prima cartella esistente.
func (folderAPI) ListDirectory(path string) FolderListing {
	roots := storageRoots()
	path = resolveStart(path, roots)

	listing := FolderListing{Path: path, Parent: parentOf(path, roots), Dirs: []string{}, Roots: roots}
	entries, err := os.ReadDir(path)
	if err != nil {
		listing.Error = "Impossibile leggere la cartella: " + err.Error()
		return listing
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || !isDirEntry(path, e) {
			continue
		}
		listing.Dirs = append(listing.Dirs, name)
	}
	sort.Slice(listing.Dirs, func(i, j int) bool {
		return strings.ToLower(listing.Dirs[i]) < strings.ToLower(listing.Dirs[j])
	})
	return listing
}

// MakeDirectory crea la sottocartella name dentro parent e ne restituisce il
// contenuto (vuoto). In caso di errore restituisce il contenuto di parent con
// il messaggio in Error.
func (f folderAPI) MakeDirectory(parent string, name string) FolderListing {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		listing := f.ListDirectory(parent)
		listing.Error = "Nome cartella non valido."
		return listing
	}
	target := filepath.Join(parent, name)
	if err := os.Mkdir(target, 0o755); err != nil && !os.IsExist(err) {
		listing := f.ListDirectory(parent)
		listing.Error = fmt.Sprintf("Impossibile creare la cartella: %v", err)
		return listing
	}
	return f.ListDirectory(target)
}

// storageRoots elenca i volumi disponibili: memoria interna e schede esterne.
func storageRoots() []FolderRoot {
	roots := []FolderRoot{}
	if isDir(internalStorage) {
		roots = append(roots, FolderRoot{Name: "Memoria interna", Path: internalStorage})
	}
	if entries, err := os.ReadDir(storageDir); err == nil {
		for _, e := range entries {
			name := e.Name()
			if name == "emulated" || name == "self" {
				continue
			}
			p := filepath.Join(storageDir, name)
			if isDir(p) {
				roots = append(roots, FolderRoot{Name: "Scheda esterna (" + name + ")", Path: p})
			}
		}
	}
	return roots
}

// resolveStart normalizza il percorso di partenza: vuoto => primo volume;
// inesistente => prima cartella esistente risalendo verso la radice.
func resolveStart(path string, roots []FolderRoot) string {
	path = strings.TrimSpace(path)
	if path == "" {
		if len(roots) > 0 {
			return roots[0].Path
		}
		return string(filepath.Separator)
	}
	path = filepath.Clean(path)
	for !isDir(path) {
		up := filepath.Dir(path)
		if up == path {
			break
		}
		path = up
	}
	return path
}

// parentOf restituisce la cartella superiore, "" se path è la radice di un
// volume (non si risale oltre: /storage e / non sono navigabili dall'utente).
func parentOf(path string, roots []FolderRoot) string {
	for _, r := range roots {
		if path == r.Path {
			return ""
		}
	}
	up := filepath.Dir(path)
	if up == path {
		return ""
	}
	return up
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// isDirEntry segue anche i link simbolici a cartelle.
func isDirEntry(parent string, e os.DirEntry) bool {
	if e.IsDir() {
		return true
	}
	if e.Type()&os.ModeSymlink != 0 {
		return isDir(filepath.Join(parent, e.Name()))
	}
	return false
}
