package playlist

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

// ytDlpReleaseURL è la base dei file di una release ufficiale di yt-dlp
// (seguita da "/<versione>/<file>"). Si scarica sempre da una versione
// precisa, non da "latest", così eseguibile e impronte arrivano dalla stessa
// release anche se ne esce una nuova fra le due richieste.
const ytDlpReleaseURL = "https://github.com/yt-dlp/yt-dlp/releases/download"

// ytDlpAsset e ytDlpSumsAsset sono l'eseguibile Windows e il file con le sue
// impronte sha256, pubblicati in ogni release.
const (
	ytDlpAsset     = "yt-dlp.exe"
	ytDlpSumsAsset = "SHA2-256SUMS"
)

// Options controlla un'esecuzione di Download.
type Options struct {
	Runner Runner // esecutore di yt-dlp (processo esterno su desktop, libreria su Android)
	URL    string // link della playlist YouTube (o, con Single, di un singolo video)
	Folder string // cartella di destinazione degli mp3

	// Single indica un link incollato dall'utente invece di una playlist
	// salvata: se punta a un video aperto dentro una playlist
	// (watch?v=...&list=...) si scarica solo quel video (--no-playlist). Un
	// link a una playlist vera e propria la scarica comunque tutta.
	Single bool

	// Workers limita quanti download avvengono in parallelo. <= 0 usa il
	// default (8): a differenza dello script bat originale (che li lanciava
	// tutti insieme senza limite), qui il limite è applicato davvero.
	Workers int

	// OnProgress, se valorizzata, viene invocata dopo ogni video scaricato
	// (con successo o meno) con (done, total).
	OnProgress func(done, total int)

	// Cancelled, se valorizzata, viene interrogata prima di avviare ogni nuovo
	// download: se ritorna true, i download non ancora avviati vengono
	// saltati. I download già in corso vengono comunque completati (non ha
	// senso interrompere un'estrazione audio a metà).
	Cancelled func() bool
}

// Result riassume l'esito di un Download.
type Result struct {
	Downloaded int
	Failed     int
	// Failures elenca i video il cui download è fallito, con il dettaglio
	// dell'errore, così la UI può mostrarli in un modale dedicato.
	Failures []Failure
}

// Failure descrive il fallimento del download di un singolo video.
type Failure struct {
	VideoID string
	Title   string // titolo dedotto in fase di enumerazione (può essere vuoto)
	URL     string
	Message string // messaggio d'errore più significativo estratto da yt-dlp
}

// IsAvailable indica se `path` punta a un file yt-dlp utilizzabile (esiste ed
// è un file, non una cartella). Percorso vuoto => non disponibile.
func IsAvailable(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// Version esegue `path --version` e restituisce la versione riportata da
// yt-dlp (stringa vuota se il file non è disponibile o il comando fallisce).
func Version(path string) string {
	if !IsAvailable(path) {
		return ""
	}
	return VersionWith(ExecRunner{Path: path})
}

// VersionWith esegue `--version` con il Runner indicato e restituisce la
// versione riportata da yt-dlp (stringa vuota se il comando fallisce).
func VersionWith(r Runner) string {
	out, _, err := r.Run([]string{"--version"})
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Install scarica l'ultima release ufficiale di yt-dlp.exe e la salva in
// `destPath`: vedi InstallVersion.
func Install(destPath string, progress Progress) error {
	version, err := LatestVersion()
	if err != nil {
		return err
	}
	return InstallVersion(destPath, version, progress)
}

// InstallVersion scarica yt-dlp.exe della release `version` e lo salva in
// `destPath` (creando le cartelle mancanti), sovrascrivendo il file
// eventualmente presente: funge quindi sia da installazione sia da
// aggiornamento. Il file scaricato deve avere l'impronta sha256 pubblicata
// nella release. Scrittura atomica: scarica su un file temporaneo nella stessa
// cartella e poi lo rinomina sul percorso finale, così un download interrotto
// o corrotto non lascia un eseguibile parziale al posto giusto (né cancella
// quello funzionante finché il nuovo non è pronto). `progress` (può essere
// nil) riceve l'avanzamento del download.
func InstallVersion(destPath, version string, progress Progress) error {
	base := ytDlpReleaseURL + "/" + version + "/"
	return installYtDlp(destPath, base+ytDlpAsset, base+ytDlpSumsAsset, progress)
}

func installYtDlp(destPath, exeURL, sumsURL string, progress Progress) error {
	if destPath == "" {
		return fmt.Errorf("percorso di destinazione non specificato")
	}
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("impossibile creare la cartella %s: %w", dir, err)
	}

	want, err := fetchSHA256(sumsURL, ytDlpAsset)
	if err != nil {
		return fmt.Errorf("download di yt-dlp fallito: %w", err)
	}

	resp, err := http.Get(exeURL)
	if err != nil {
		return fmt.Errorf("download di yt-dlp fallito: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download di yt-dlp fallito: HTTP %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp(dir, "yt-dlp-*.tmp")
	if err != nil {
		return fmt.Errorf("impossibile creare il file temporaneo: %w", err)
	}
	tmpPath := tmp.Name()

	hash := sha256.New()
	pw := newProgressWriter(progress, PhaseDownload, resp.ContentLength)
	_, copyErr := io.Copy(io.MultiWriter(tmp, hash), io.TeeReader(resp.Body, pw))
	closeErr := tmp.Close()
	if copyErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("scrittura di yt-dlp fallita: %w", copyErr)
	}
	if closeErr != nil {
		os.Remove(tmpPath)
		return closeErr
	}
	if hex.EncodeToString(hash.Sum(nil)) != want {
		os.Remove(tmpPath)
		return fmt.Errorf("download di yt-dlp fallito: %w", errChecksumMismatch)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		// Il file esistente potrebbe essere di sola lettura/in uso: proviamo a
		// rimuoverlo e a rinominare di nuovo (stessa strategia di settings).
		os.Remove(destPath)
		if err2 := os.Rename(tmpPath, destPath); err2 != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("installazione di yt-dlp fallita: %w", err2)
		}
	}
	return nil
}

// TempDirName è la sottocartella nascosta della cartella dei download in cui
// yt-dlp scrive i file intermedi (i .part e l'audio originale prima della
// conversione in mp3): nella cartella arriva solo l'mp3 finito. Così un
// download interrotto (rete, errore di ffmpeg, app chiusa a metà) non lascia
// accanto ai brani residui che la scansione, che elenca solo gli mp3, non
// mostrerebbe mai, e la cartella non riceve la raffica di scritture
// intermedie. Ogni video ha una sua sottocartella, eliminata a fine download.
const TempDirName = ".renamemusic-download"

// CleanTempDir elimina la cartella dei file intermedi di yt-dlp in folder,
// con gli eventuali residui di un download interrotto. Va chiamata solo
// quando in quella cartella non c'è un download in corso.
func CleanTempDir(folder string) error {
	if folder == "" {
		return nil
	}
	return os.RemoveAll(filepath.Join(folder, TempDirName))
}

// Download scarica in mp3 tutti i video di una playlist YouTube: enumera gli
// ID dei video (yt-dlp --flat-playlist --print id) e poi scarica ogni video
// con concorrenza limitata a Options.Workers. A differenza dello script bat
// originale (che lanciava i processi in background e attendeva con un polling
// su tasklist), ogni esecuzione di yt-dlp viene attesa esplicitamente
// (Runner.Run è bloccante): Download ritorna solo quando TUTTI i download sono
// conclusi.
func Download(opts Options) (Result, error) {
	if opts.Runner == nil {
		return Result{}, fmt.Errorf("yt-dlp non configurato")
	}
	// Si parte senza i residui di un download interrotto e, a fine download
	// (anche annullato: i download avviati si attendono), non resta nulla.
	_ = CleanTempDir(opts.Folder)
	defer CleanTempDir(opts.Folder)

	videos, err := listVideos(opts.Runner, opts.URL, opts.Single)
	if err != nil {
		return Result{}, err
	}
	total := len(videos)
	if total == 0 {
		if opts.Single {
			return Result{}, fmt.Errorf("nessun video trovato al link")
		}
		return Result{}, fmt.Errorf("nessun video trovato nella playlist")
	}

	// Notifica subito il totale (0 completati): così la UI può mostrare la barra
	// di avanzamento con il totale delle canzoni non appena l'enumerazione della
	// playlist è finita, senza aspettare il primo download concluso.
	if opts.OnProgress != nil {
		opts.OnProgress(0, total)
	}

	workers := opts.Workers
	if workers <= 0 {
		workers = 8
	}

	var (
		wg         sync.WaitGroup
		sem        = make(chan struct{}, workers)
		downloaded int32
		failed     int32
		done       int32
		mu         sync.Mutex
		failures   []Failure
	)

	for _, v := range videos {
		if opts.Cancelled != nil && opts.Cancelled() {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(info videoInfo) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := safeDownloadOne(opts.Runner, opts.Folder, info.id); err != nil {
				atomic.AddInt32(&failed, 1)
				mu.Lock()
				failures = append(failures, Failure{
					VideoID: info.id,
					Title:   info.title,
					URL:     "https://www.youtube.com/watch?v=" + info.id,
					Message: err.Error(),
				})
				mu.Unlock()
			} else {
				atomic.AddInt32(&downloaded, 1)
			}
			d := atomic.AddInt32(&done, 1)
			if opts.OnProgress != nil {
				opts.OnProgress(int(d), total)
			}
		}(v)
	}

	wg.Wait()
	return Result{Downloaded: int(downloaded), Failed: int(failed), Failures: failures}, nil
}

// videoInfo è l'ID + titolo di un video enumerato dalla playlist.
type videoInfo struct {
	id    string
	title string
}

// listVideos enumera i video di una playlist senza scaricare nulla
// (--flat-playlist), stampando ID e titolo separati da un tab: una riga per
// video. Il titolo serve solo a rendere leggibile l'eventuale elenco di errori.
// Con single (link di un video) l'enumerazione restituisce solo quel video,
// anche se il link lo apre dentro una playlist.
func listVideos(r Runner, url string, single bool) ([]videoInfo, error) {
	args := []string{"--flat-playlist", "--print", "%(id)s\t%(title)s", url}
	what := "estrazione playlist fallita"
	if single {
		args = append([]string{args[0], "--no-playlist"}, args[1:]...)
		what = "lettura del link fallita"
	}
	out, stderr, err := r.Run(args)
	if err != nil {
		if msg := extractYtDlpError(string(stderr)); msg != "" {
			return nil, fmt.Errorf("%s: %s", what, msg)
		}
		return nil, fmt.Errorf("%s: %w", what, err)
	}

	var videos []videoInfo
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		id, title, _ := strings.Cut(line, "\t")
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		videos = append(videos, videoInfo{id: id, title: strings.TrimSpace(title)})
	}
	return videos, nil
}

// safeDownloadOne è downloadOne con un eventuale panic (es. nel ponte verso
// youtubedl-android) trasformato nell'errore del singolo brano: gira in una
// goroutine, dove un panic chiuderebbe l'intera app.
func safeDownloadOne(r Runner, folder, videoID string) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("errore interno: %v", p)
		}
	}()
	return downloadOne(r, folder, videoID)
}

// downloadOne scarica ed estrae in mp3 un singolo video, con nome file basato
// sul titolo (stesse opzioni dello script bat originale). I file intermedi
// stanno in una sottocartella del video dentro TempDirName (-P temp:), che si
// elimina comunque vada; yt-dlp sposta in folder (-P home:) solo l'mp3 finito.
// Il modello di -o deve restare relativo: uno assoluto farebbe ignorare a
// yt-dlp la cartella temporanea. In caso di errore cattura lo stderr di
// yt-dlp e ne restituisce il messaggio più significativo.
func downloadOne(r Runner, folder, videoID string) error {
	temp := filepath.Join(folder, TempDirName, videoID)
	defer os.RemoveAll(temp)
	_, stderr, err := r.Run([]string{
		"-x", "--audio-format", "mp3",
		"--no-mtime", "--windows-filenames", "--trim-filenames", "200",
		"-P", "home:" + folder,
		"-P", "temp:" + temp,
		"-o", "%(title)s.%(ext)s",
		"https://www.youtube.com/watch?v=" + videoID,
	})
	if err != nil {
		if msg := extractYtDlpError(string(stderr)); msg != "" {
			return fmt.Errorf("%s", msg)
		}
		return err
	}
	return nil
}

// extractYtDlpError ricava dallo stderr di yt-dlp il messaggio d'errore più
// utile: preferisce l'ultima riga che inizia con "ERROR:", altrimenti l'ultima
// riga non vuota. Restituisce "" se lo stderr è vuoto.
func extractYtDlpError(stderr string) string {
	var last, errLine string
	scanner := bufio.NewScanner(strings.NewReader(stderr))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		l := strings.TrimSpace(scanner.Text())
		if l == "" {
			continue
		}
		last = l
		if strings.HasPrefix(l, "ERROR:") {
			errLine = l
		}
	}
	if errLine != "" {
		return errLine
	}
	return last
}
