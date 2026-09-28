package playlist

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ffmpegDownloadURL è la build Windows di ffmpeg pubblicata dal progetto yt-dlp
// (la stessa consigliata nel suo README, con le patch per yt-dlp): il tag
// "latest" punta sempre alla build più recente. Lo zip contiene
// <cartella>/bin/ffmpeg.exe e ffprobe.exe, gli unici file che servono.
const ffmpegDownloadURL = "https://github.com/yt-dlp/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"

// ffmpegBinaries sono gli eseguibili estratti dallo zip: yt-dlp usa ffmpeg per
// l'estrazione dell'audio in mp3 e ffprobe per analizzare i file scaricati.
var ffmpegBinaries = []string{"ffmpeg.exe", "ffprobe.exe"}

// FFmpegInstalled indica se in `dir` (la cartella gestita dall'app) ci sono
// tutti gli eseguibili di ffmpeg. Cartella vuota => non installato.
func FFmpegInstalled(dir string) bool {
	if dir == "" {
		return false
	}
	for _, name := range ffmpegBinaries {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || info.IsDir() {
			return false
		}
	}
	return true
}

// FFmpegInPath indica se ffmpeg è già installato nel sistema (nel PATH): in tal
// caso yt-dlp lo trova da sé e l'app non ha bisogno di scaricarne una copia.
func FFmpegInPath() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

// InstallFFmpeg scarica l'ultima build di ffmpeg ed estrae ffmpeg.exe e
// ffprobe.exe in `dir` (creandola se manca), sovrascrivendo quelli presenti.
// Lo zip (~200 MB) viene scaricato su un file temporaneo nella stessa cartella
// e rimosso al termine; ogni eseguibile è scritto in modo atomico (temporaneo +
// rename), così un'installazione interrotta non lascia file parziali al posto
// giusto. `progress` (può essere nil) riceve l'avanzamento del download e poi
// quello dell'estrazione.
func InstallFFmpeg(dir string, progress Progress) error {
	if dir == "" {
		return fmt.Errorf("cartella di destinazione non specificata")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("impossibile creare la cartella %s: %w", dir, err)
	}

	resp, err := http.Get(ffmpegDownloadURL)
	if err != nil {
		return fmt.Errorf("download di ffmpeg fallito: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download di ffmpeg fallito: HTTP %d", resp.StatusCode)
	}

	// archive/zip richiede un io.ReaderAt: lo zip va prima salvato su disco.
	tmp, err := os.CreateTemp(dir, "ffmpeg-*.zip.tmp")
	if err != nil {
		return fmt.Errorf("impossibile creare il file temporaneo: %w", err)
	}
	zipPath := tmp.Name()
	defer os.Remove(zipPath)

	pw := newProgressWriter(progress, PhaseDownload, resp.ContentLength)
	_, copyErr := io.Copy(tmp, io.TeeReader(resp.Body, pw))
	closeErr := tmp.Close()
	if copyErr != nil {
		return fmt.Errorf("download di ffmpeg fallito: %w", copyErr)
	}
	if closeErr != nil {
		return closeErr
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("archivio di ffmpeg non valido: %w", err)
	}
	defer zr.Close()

	// Prima si trovano tutti i file, così l'avanzamento dell'estrazione ha un
	// totale unico (somma delle dimensioni estratte).
	files := make([]*zip.File, len(ffmpegBinaries))
	var total int64
	for i, name := range ffmpegBinaries {
		f := findInZip(&zr.Reader, name)
		if f == nil {
			return fmt.Errorf("archivio di ffmpeg non valido: %s non trovato", name)
		}
		files[i] = f
		total += int64(f.UncompressedSize64)
	}

	ew := newProgressWriter(progress, PhaseExtract, total)
	for i, name := range ffmpegBinaries {
		if err := extractAtomic(files[i], filepath.Join(dir, name), ew); err != nil {
			return fmt.Errorf("installazione di ffmpeg fallita: %w", err)
		}
	}
	return nil
}

// findInZip cerca nello zip il file `name` dentro una cartella "bin" (la radice
// dell'archivio cambia a ogni build, es. ffmpeg-master-latest-win64-gpl/bin/).
func findInZip(zr *zip.Reader, name string) *zip.File {
	for _, f := range zr.File {
		p := strings.ToLower(strings.ReplaceAll(f.Name, "\\", "/"))
		if strings.HasSuffix(p, "/bin/"+name) {
			return f
		}
	}
	return nil
}

// extractAtomic estrae `f` su un temporaneo accanto a `dest` e lo rinomina su
// `dest` solo a estrazione completata (stessa strategia di Install). I byte
// estratti passano anche da `counter`, per l'avanzamento.
func extractAtomic(f *zip.File, dest string, counter io.Writer) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dest), filepath.Base(dest)+"-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	_, copyErr := io.Copy(tmp, io.TeeReader(rc, counter))
	closeErr := tmp.Close()
	if copyErr != nil {
		os.Remove(tmpPath)
		return copyErr
	}
	if closeErr != nil {
		os.Remove(tmpPath)
		return closeErr
	}

	if err := os.Rename(tmpPath, dest); err != nil {
		os.Remove(dest)
		if err2 := os.Rename(tmpPath, dest); err2 != nil {
			os.Remove(tmpPath)
			return err2
		}
	}
	return nil
}
