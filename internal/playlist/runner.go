package playlist

import (
	"bytes"
	"os/exec"
)

// Runner esegue yt-dlp con gli argomenti indicati e ne restituisce stdout e
// stderr. Astrae il "come" gira yt-dlp, così la logica di Download (enumerazione,
// concorrenza, estrazione degli errori) è la stessa su tutte le piattaforme: su
// desktop è un processo esterno (ExecRunner), su Android è la libreria
// youtubedl-android (Python incorporato nell'APK) raggiunta tramite il bridge.
//
// err != nil indica un'esecuzione fallita (processo non avviato o exit code
// diverso da zero); stderr resta comunque valorizzato quando disponibile, così
// il chiamante può estrarne il messaggio più significativo.
type Runner interface {
	Run(args []string) (stdout, stderr []byte, err error)
}

// ExecRunner esegue l'eseguibile yt-dlp indicato da Path come processo esterno,
// senza finestra di console (vedi hideWindow).
type ExecRunner struct {
	Path string
}

func (r ExecRunner) Run(args []string) ([]byte, []byte, error) {
	cmd := exec.Command(r.Path, args...)
	hideWindow(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}
