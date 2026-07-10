package playlist

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

// ytDlpDownloadURL è l'ultima release ufficiale di yt-dlp per Windows: il
// redirect "latest" punta sempre alla versione più recente, così l'installazione
// dalla UI scarica sempre l'aggiornamento corrente.
const ytDlpDownloadURL = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe"

// Options controlla un'esecuzione di Download.
type Options struct {
	YtDlpPath string // percorso dell'eseguibile yt-dlp da usare
	URL       string // link della playlist YouTube
	Folder    string // cartella di destinazione degli mp3

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
	cmd := exec.Command(path, "--version")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Install scarica l'ultima release ufficiale di yt-dlp.exe e la salva in
// `destPath` (creando le cartelle mancanti), sovrascrivendo il file eventualmente
// presente: funge quindi sia da installazione sia da aggiornamento. Scrittura
// atomica: scarica su un file temporaneo nella stessa cartella e poi lo rinomina
// sul percorso finale, così un download interrotto non lascia un eseguibile
// parziale al posto giusto (né cancella quello funzionante finché il nuovo non è
// pronto).
func Install(destPath string) error {
	if destPath == "" {
		return fmt.Errorf("percorso di destinazione non specificato")
	}
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("impossibile creare la cartella %s: %w", dir, err)
	}

	resp, err := http.Get(ytDlpDownloadURL)
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

	_, copyErr := io.Copy(tmp, resp.Body)
	closeErr := tmp.Close()
	if copyErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("scrittura di yt-dlp fallita: %w", copyErr)
	}
	if closeErr != nil {
		os.Remove(tmpPath)
		return closeErr
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

// Download scarica in mp3 tutti i video di una playlist YouTube: enumera gli
// ID dei video (yt-dlp --flat-playlist --print id) e poi scarica ogni video
// con concorrenza limitata a Options.Workers. A differenza dello script bat
// originale (che lanciava i processi in background e attendeva con un polling
// su tasklist), ogni processo yt-dlp viene atteso esplicitamente con
// cmd.Run(): Download ritorna solo quando TUTTI i download sono conclusi.
func Download(opts Options) (Result, error) {
	ids, err := listVideoIDs(opts.YtDlpPath, opts.URL)
	if err != nil {
		return Result{}, err
	}
	total := len(ids)
	if total == 0 {
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
	)

	for _, id := range ids {
		if opts.Cancelled != nil && opts.Cancelled() {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(videoID string) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := downloadOne(opts.YtDlpPath, opts.Folder, videoID); err != nil {
				atomic.AddInt32(&failed, 1)
			} else {
				atomic.AddInt32(&downloaded, 1)
			}
			d := atomic.AddInt32(&done, 1)
			if opts.OnProgress != nil {
				opts.OnProgress(int(d), total)
			}
		}(id)
	}

	wg.Wait()
	return Result{Downloaded: int(downloaded), Failed: int(failed)}, nil
}

// listVideoIDs enumera gli ID video di una playlist senza scaricare nulla
// (--flat-playlist --print id), un ID per riga in stdout.
func listVideoIDs(ytdlp, url string) ([]string, error) {
	cmd := exec.Command(ytdlp, "--flat-playlist", "--print", "id", url)
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("estrazione playlist fallita: %w", err)
	}

	var ids []string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			ids = append(ids, line)
		}
	}
	return ids, nil
}

// downloadOne scarica ed estrae in mp3 un singolo video, con nome file basato
// sul titolo (stesse opzioni dello script bat originale).
func downloadOne(ytdlp, folder, videoID string) error {
	out := filepath.Join(folder, "%(title)s.%(ext)s")
	cmd := exec.Command(ytdlp,
		"-x", "--audio-format", "mp3",
		"--no-mtime", "--windows-filenames", "--trim-filenames", "200",
		"-o", out,
		"https://www.youtube.com/watch?v="+videoID,
	)
	hideWindow(cmd)
	return cmd.Run()
}
