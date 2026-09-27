package mobile

import (
	"encoding/json"
	"errors"
	"sync"

	"renamemusic/internal/playlist"
)

// ManagedLabel è il "percorso" di yt-dlp mostrato in UI su Android: yt-dlp non
// è un file scelto dall'utente ma è integrato nell'APK (youtubedl-android).
const ManagedLabel = "Integrato nell'app (youtubedl-android)"

// androidWorkers è il numero di download paralleli su Android: ogni download è
// un processo Python + ffmpeg, pesante per un telefono (su desktop sono 8).
const androidWorkers = 3

// androidYtDlp implementa core.YtDlp sopra youtubedl-android. Disponibilità e
// versione sono in cache (aggiornate da load): il core le legge sotto il proprio
// lock e su Android `yt-dlp --version` avvia Python, quindi non va eseguito lì.
type androidYtDlp struct {
	host Host

	mu      sync.Mutex
	ready   bool
	version string
}

// load rilegge disponibilità e versione. Esegue yt-dlp: va chiamata fuori dal
// lock del core.
func (y *androidYtDlp) load() {
	ready := y.host.YtDlpReady()
	version := ""
	if ready {
		version = playlist.VersionWith(bridgeRunner{host: y.host})
	}
	y.mu.Lock()
	y.ready = ready
	y.version = version
	y.mu.Unlock()
}

func (y *androidYtDlp) ManagedPath() (string, error) { return ManagedLabel, nil }

func (y *androidYtDlp) Available(string) bool {
	y.mu.Lock()
	defer y.mu.Unlock()
	return y.ready
}

func (y *androidYtDlp) Version(string) string {
	y.mu.Lock()
	defer y.mu.Unlock()
	return y.version
}

// Install aggiorna yt-dlp all'ultima versione stabile (su Android è sempre
// presente: "installare" equivale ad aggiornare).
func (y *androidYtDlp) Install(string) error {
	if msg := y.host.YtDlpUpdate(); msg != "" {
		y.load()
		return errors.New(msg)
	}
	y.load()
	return nil
}

func (y *androidYtDlp) Uninstall(string) error {
	return errors.New("su Android yt-dlp è integrato nell'app e non può essere rimosso")
}

func (y *androidYtDlp) Runner(string) playlist.Runner { return bridgeRunner{host: y.host} }

func (y *androidYtDlp) Workers() int { return androidWorkers }

// bridgeRunner esegue yt-dlp attraverso Host.YtDlpRun (youtubedl-android).
type bridgeRunner struct {
	host Host
}

type runResult struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Error  string `json:"error"`
}

func (r bridgeRunner) Run(args []string) ([]byte, []byte, error) {
	in, err := json.Marshal(args)
	if err != nil {
		return nil, nil, err
	}
	var res runResult
	if err := json.Unmarshal([]byte(r.host.YtDlpRun(string(in))), &res); err != nil {
		return nil, nil, errors.New("risposta di yt-dlp non valida: " + err.Error())
	}
	if res.Error != "" {
		return []byte(res.Stdout), []byte(res.Stderr), errors.New(res.Error)
	}
	return []byte(res.Stdout), []byte(res.Stderr), nil
}
