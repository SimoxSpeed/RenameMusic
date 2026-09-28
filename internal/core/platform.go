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
	// Install scarica/aggiorna yt-dlp al percorso indicato; `progress` (può
	// essere nil, o non essere mai chiamata se la piattaforma non la supporta)
	// riceve l'avanzamento.
	Install(path string, progress playlist.Progress) error
	// Uninstall rimuove la copia gestita (con l'eventuale ffmpeg gestito);
	// nessun errore se era già assente.
	Uninstall(path string) error
	// FFmpegAvailable indica se yt-dlp ha a disposizione ffmpeg, necessario
	// per estrarre l'audio in mp3 (copia gestita dall'app o ffmpeg di sistema).
	FFmpegAvailable() bool
	// InstallFFmpeg scarica/aggiorna la copia di ffmpeg gestita dall'app, con
	// l'avanzamento su `progress` come Install.
	InstallFFmpeg(progress playlist.Progress) error
	// Runner restituisce l'esecutore di yt-dlp per il percorso indicato.
	Runner(path string) playlist.Runner
	// Workers è il numero di download paralleli (<= 0 => default di playlist).
	Workers() int
}

// Updater astrae l'installazione di un aggiornamento dell'app, che dipende
// dalla piattaforma: il controllo della nuova versione e il download sono
// comuni (package update), mentre su desktop si sostituisce l'eseguibile e si
// riavvia, su Android si apre l'installer di sistema sull'APK scaricato.
type Updater interface {
	// Asset è il nome del file, tra gli allegati della release GitHub, adatto
	// a questa piattaforma (es. "RenameMusic.exe").
	Asset() string
	// DownloadPath è dove salvare il file scaricato.
	DownloadPath() (string, error)
	// Apply installa il file scaricato in path e restituisce il messaggio da
	// mostrare all'utente (es. riavvio in corso).
	Apply(path string) (string, error)
	// Cleanup rimuove i residui di un aggiornamento precedente. È chiamata una
	// volta all'avvio, prima del primo controllo.
	Cleanup()
}

// ExecYtDlp è la gestione desktop di yt-dlp: un eseguibile esterno lanciato
// come processo, con la copia gestita dentro la cartella di configurazione.
// Anche ffmpeg (che yt-dlp usa per l'mp3) ha una copia gestita, in
// %AppData%\RenameMusic\ffmpeg, usata sia con yt-dlp gestito sia con quello
// scelto a mano; in sua assenza yt-dlp usa l'eventuale ffmpeg di sistema.
type ExecYtDlp struct{}

func (ExecYtDlp) ManagedPath() (string, error) { return settings.YtDlpManagedPath() }
func (ExecYtDlp) Available(path string) bool   { return playlist.IsAvailable(path) }
func (ExecYtDlp) Version(path string) string   { return playlist.Version(path) }
func (ExecYtDlp) Workers() int                 { return 0 }

func (ExecYtDlp) Install(path string, progress playlist.Progress) error {
	return playlist.Install(path, progress)
}

func (ExecYtDlp) Uninstall(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	if dir, err := settings.FFmpegManagedDir(); err == nil {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	return nil
}

func (ExecYtDlp) FFmpegAvailable() bool {
	return managedFFmpegDir() != "" || playlist.FFmpegInPath()
}

func (ExecYtDlp) InstallFFmpeg(progress playlist.Progress) error {
	dir, err := settings.FFmpegManagedDir()
	if err != nil {
		return err
	}
	return playlist.InstallFFmpeg(dir, progress)
}

func (ExecYtDlp) Runner(path string) playlist.Runner {
	return playlist.ExecRunner{Path: path, FFmpegDir: managedFFmpegDir()}
}

// managedFFmpegDir restituisce la cartella della copia gestita di ffmpeg se è
// installata, altrimenti "".
func managedFFmpegDir() string {
	dir, err := settings.FFmpegManagedDir()
	if err != nil || !playlist.FFmpegInstalled(dir) {
		return ""
	}
	return dir
}

// errNoHost è restituito dai selettori quando il core gira senza Host (test).
var errNoHost = errors.New("funzione non disponibile su questa piattaforma")

// nopHost è l'Host usato quando nessuno è stato fornito (es. nei test): gli
// eventi vengono scartati e i selettori non sono disponibili.
type nopHost struct{}

func (nopHost) Emit(string, any)                                {}
func (nopHost) ChooseDirectory(string) (string, error)          { return "", errNoHost }
func (nopHost) ChooseFile(string, []FileFilter) (string, error) { return "", errNoHost }
func (nopHost) OpenFolder(string) error                         { return errNoHost }
