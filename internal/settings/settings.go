package settings

import (
	"encoding/json"
	"os"
	"path/filepath"

	"renamemusic/internal/rules"
)

const (
	appDirName   = "RenameMusic"
	currentFile  = "config.json"
	defaultsFile = "defaults.json"
	stateFile    = "state.json"
	filePerm     = 0o644
	dirPermMode  = 0o755
)

// State conserva lo stato non-regola persistito: cartella di partenza,
// opzioni di destinazione ed eliminazione originali.
type State struct {
	LastFolder              string `json:"lastFolder"`
	DestinationSameAsSource bool   `json:"destinationSameAsSource"`
	DestinationFolder       string `json:"destinationFolder"`
	DeleteOriginals         bool   `json:"deleteOriginals"`
}

// DefaultState è lo stato al primo avvio: nessuna cartella, destinazione = partenza,
// nessuna eliminazione degli originali.
func DefaultState() State {
	return State{DestinationSameAsSource: true}
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
	return os.WriteFile(path, data, filePerm)
}

// LoadConfig / SaveConfig gestiscono le regole correnti (attive).
func LoadConfig() (rules.Config, bool, error) { return load(currentFile) }
func SaveConfig(cfg rules.Config) error       { return save(currentFile, cfg) }

// LoadDefaults / SaveDefaults gestiscono i default (editabili e persistiti).
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
	return os.WriteFile(path, data, filePerm)
}
