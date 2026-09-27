package core

import (
	"errors"
	"os"

	"renamemusic/internal/playlist"
	"renamemusic/internal/settings"
)

// Host raccoglie i servizi che dipendono dalla piattaforma su cui gira il core:
// la finestra desktop Wails oppure l'app Android. Il core non conosce né Wails
// né Android: chiede all'Host di notificare la UI, aprire i selettori di sistema
// e il file manager.
type Host interface {
	// Emit invia alla UI un evento asincrono (es. EventWatchChanged) con il
	// payload indicato, che la UI riceve già deserializzato.
	Emit(event string, payload any)
	// ChooseDirectory apre il selettore cartella di sistema; "" se annullato.
	ChooseDirectory(title string) (string, error)
	// ChooseFile apre il selettore file di sistema; "" se annullato.
	ChooseFile(title string, filters []FileFilter) (string, error)
	// OpenFolder apre la cartella nel file manager di sistema.
	OpenFolder(path string) error
}

// FileFilter è un filtro del selettore file (es. "Eseguibili (*.exe)", "*.exe").
type FileFilter struct {
	DisplayName string
	Pattern     string
}

// YtDlp astrae dove si trova e come gira yt-dlp. Su desktop è un eseguibile
// esterno (copia gestita in %AppData%\RenameMusic o percorso scelto a mano, vedi
// ExecYtDlp); su Android è incorporato nell'APK (youtubedl-android) e `path` non
// ha significato. I metodi ricevono il percorso effettivo calcolato dal core.
type YtDlp interface {
	// ManagedPath è il percorso della copia gestita dall'app.
	ManagedPath() (string, error)
	// Available indica se yt-dlp è utilizzabile al percorso indicato.
	Available(path string) bool
	// Version restituisce la versione di yt-dlp ("" se non disponibile).
	// Può eseguire yt-dlp: va chiamata solo quando lo stato può essere cambiato.
	Version(path string) string
	// Install scarica/aggiorna yt-dlp al percorso indicato.
	Install(path string) error
	// Uninstall rimuove la copia gestita; nessun errore se era già assente.
	Uninstall(path string) error
	// Runner restituisce l'esecutore di yt-dlp per il percorso indicato.
	Runner(path string) playlist.Runner
	// Workers è il numero di download paralleli (<= 0 => default di playlist).
	Workers() int
}

// ExecYtDlp è la gestione desktop di yt-dlp: un eseguibile esterno lanciato
// come processo, con la copia gestita dentro la cartella di configurazione.
type ExecYtDlp struct{}

func (ExecYtDlp) ManagedPath() (string, error) { return settings.YtDlpManagedPath() }
func (ExecYtDlp) Available(path string) bool   { return playlist.IsAvailable(path) }
func (ExecYtDlp) Version(path string) string   { return playlist.Version(path) }
func (ExecYtDlp) Install(path string) error    { return playlist.Install(path) }
func (ExecYtDlp) Workers() int                 { return 0 }

func (ExecYtDlp) Uninstall(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (ExecYtDlp) Runner(path string) playlist.Runner { return playlist.ExecRunner{Path: path} }

// errNoHost è restituito dai selettori quando il core gira senza Host (test).
var errNoHost = errors.New("funzione non disponibile su questa piattaforma")

// nopHost è l'Host usato quando nessuno è stato fornito (es. nei test): gli
// eventi vengono scartati e i selettori non sono disponibili.
type nopHost struct{}

func (nopHost) Emit(string, any)                                {}
func (nopHost) ChooseDirectory(string) (string, error)          { return "", errNoHost }
func (nopHost) ChooseFile(string, []FileFilter) (string, error) { return "", errNoHost }
func (nopHost) OpenFolder(string) error                         { return errNoHost }
