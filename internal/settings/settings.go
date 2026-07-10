package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"renamemusic/internal/playlist"
	"renamemusic/internal/rules"
)

const (
	appDirName           = "RenameMusic"
	currentFile          = "config.json"
	defaultsFile         = "defaults.json"
	stateFile            = "state.json"
	playlistsFile        = "playlists.json"
	defaultPlaylistsFile = "defaults-playlists.json"
	filePerm             = 0o644
	dirPermMode          = 0o755

	// tempSuffix marca i file temporanei della scrittura atomica: un crash tra
	// la scrittura del temp e il rename può lasciarli orfani nella cartella di
	// configurazione. Il suffisso specifico consente a CleanTempFiles di
	// rimuovere solo i nostri residui.
	tempSuffix = ".renamemusic.tmp"
)

// State conserva lo stato non-regola persistito: cartella di partenza,
// opzioni di destinazione ed eliminazione originali.
type State struct {
	LastFolder              string `json:"lastFolder"`
	DestinationSameAsSource bool   `json:"destinationSameAsSource"`
	DestinationFolder       string `json:"destinationFolder"`
	DeleteOriginals         bool   `json:"deleteOriginals"`
	WatchEnabled            bool   `json:"watchEnabled"`

	// YtDlpManaged: se true l'app gestisce da sé yt-dlp, usando (e aggiornando)
	// una propria copia in %AppData%\RenameMusic. Se false si usa YtDlpPath, il
	// percorso all'eseguibile yt-dlp scelto a mano dall'utente.
	YtDlpManaged bool   `json:"ytDlpManaged"`
	YtDlpPath    string `json:"ytDlpPath"`
}

// DefaultState è lo stato al primo avvio: nessuna cartella, destinazione =
// partenza, nessuna eliminazione degli originali, yt-dlp gestito dall'app.
func DefaultState() State {
	return State{DestinationSameAsSource: true, YtDlpManaged: true}
}

// YtDlpManagedPath restituisce il percorso della copia di yt-dlp gestita
// dall'app: dentro la cartella di configurazione (%AppData%\RenameMusic), così
// è scrivibile senza permessi di amministratore anche se l'app fosse installata
// in una cartella protetta.
func YtDlpManagedPath() (string, error) {
	return pathFor("yt-dlp.exe")
}

// Dir restituisce la cartella di configurazione (es. %AppData%\RenameMusic su Windows).
func Dir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appDirName), nil
}

func pathFor(name string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

// load legge un file di configurazione. Se manca restituisce il seed di fabbrica
// (existed=false). I campi mancanti nel JSON mantengono i valori di fabbrica.
func load(name string) (cfg rules.Config, existed bool, err error) {
	cfg = rules.FactoryConfig()

	path, err := pathFor(name)
	if err != nil {
		return cfg, false, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, false, nil
		}
		return cfg, false, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		// File corrotto: torniamo al seed di fabbrica senza propagare in modo fatale.
		return rules.FactoryConfig(), true, err
	}
	return cfg, true, nil
}

func save(name string, cfg rules.Config) error {
	path, err := pathFor(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), dirPermMode); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, filePerm)
}

// writeFileAtomic scrive data su path in modo atomico: scrive prima su un file
// temporaneo e lo rinomina su path solo a scrittura completata. Evita che un
// crash o due istanze dell'app in scrittura contemporanea lascino un JSON di
// configurazione troncato/corrotto. Se il rename diretto fallisce (es. file di
// sola lettura su Windows) si riprova rimuovendo prima il file esistente.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + tempSuffix
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(path)
		if err2 := os.Rename(tmp, path); err2 != nil {
			_ = os.Remove(tmp)
			return err2
		}
	}
	return nil
}

// CleanTempFiles rimuove i file temporanei orfani dalla cartella di
// configurazione (%AppData%\RenameMusic). Restituisce quanti ne ha rimossi.
func CleanTempFiles() int {
	dir, err := Dir()
	if err != nil {
		return 0
	}
	return cleanTempFilesIn(dir)
}

// cleanTempFilesIn rimuove i file col suffisso marcato dalla cartella indicata.
// Isolata da CleanTempFiles per essere testabile su una cartella temporanea.
func cleanTempFilesIn(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), tempSuffix) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err == nil {
			removed++
		}
	}
	return removed
}

// LoadConfig / SaveConfig gestiscono le regole correnti (attive).
func LoadConfig() (rules.Config, bool, error) { return load(currentFile) }
func SaveConfig(cfg rules.Config) error       { return save(currentFile, cfg) }

// LoadDefaults / SaveDefaults gestiscono i predefiniti (editabili e persistiti).
func LoadDefaults() (rules.Config, bool, error) { return load(defaultsFile) }
func SaveDefaults(cfg rules.Config) error       { return save(defaultsFile, cfg) }

// LoadState restituisce lo stato persistito (DefaultState se mai salvato).
func LoadState() (State, error) {
	s := DefaultState()

	path, err := pathFor(stateFile)
	if err != nil {
		return s, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return DefaultState(), err
	}
	return s, nil
}

// SaveState persiste lo stato non-regola.
func SaveState(s State) error {
	path, err := pathFor(stateFile)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), dirPermMode); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, filePerm)
}

// LoadPlaylists restituisce le playlist YouTube salvate (correnti), nil se non
// ancora salvate. Non sono "regole" di rinomina, ma — come le regole — hanno un
// preset predefinito a parte (defaults-playlists.json): "Salva come predefinito"
// aggiorna quello, "Ripristina predefiniti" lo ripristina qui.
func LoadPlaylists() ([]playlist.Playlist, error) { return loadPlaylists(playlistsFile) }

// SavePlaylists persiste le playlist YouTube correnti.
func SavePlaylists(list []playlist.Playlist) error { return savePlaylists(playlistsFile, list) }

// LoadDefaultPlaylists / SaveDefaultPlaylists gestiscono il preset predefinito
// (editabile e persistito) delle playlist, analogo a LoadDefaults/SaveDefaults
// per le regole.
func LoadDefaultPlaylists() ([]playlist.Playlist, error) { return loadPlaylists(defaultPlaylistsFile) }
func SaveDefaultPlaylists(list []playlist.Playlist) error {
	return savePlaylists(defaultPlaylistsFile, list)
}

func loadPlaylists(name string) ([]playlist.Playlist, error) {
	path, err := pathFor(name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var list []playlist.Playlist
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func savePlaylists(name string, list []playlist.Playlist) error {
	path, err := pathFor(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), dirPermMode); err != nil {
		return err
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, filePerm)
}
