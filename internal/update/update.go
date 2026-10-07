// Package update controlla se è uscita una nuova versione dell'app e la
// scarica. Le versioni sono le release GitHub del repository (tag "vX.Y.Z"),
// con allegati i file per ogni piattaforma (RenameMusic.exe, gli APK per ABI):
// il confronto e il download sono comuni a desktop e Android, mentre
// l'installazione vera e propria dipende dalla piattaforma (vedi core.Updater).
package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"renamemusic/internal/playlist"
)

// Version è la versione di questa build. Va incrementata a ogni release: il tag
// della release GitHub corrispondente deve essere "v" + Version (es. v1.1.0).
// build-apk.ps1 la legge da qui per versionName/versionCode dell'APK.
const Version = "4.0.0"

// Repo è il repository GitHub (pubblico) da cui arrivano le release.
const Repo = "SimoxSpeed/RenameMusic"

// latestURL è l'endpoint dell'ultima release pubblicata (esclude bozze e
// pre-release). Variabile per poterlo sostituire nei test; la variabile
// d'ambiente RENAMEMUSIC_UPDATE_URL lo sostituisce per provare l'aggiornamento
// con un server locale che imita l'API di GitHub.
var latestURL = "https://api.github.com/repos/" + Repo + "/releases/latest"

func init() {
	if u := os.Getenv("RENAMEMUSIC_UPDATE_URL"); u != "" {
		latestURL = u
	}
}

// ErrNoRelease indica che il repository non ha ancora release pubblicate.
var ErrNoRelease = errors.New("nessuna release pubblicata")

// Release è una versione pubblicata su GitHub.
type Release struct {
	Version string // senza la "v" iniziale del tag
	Notes   string // descrizione della release (testo/markdown grezzo)
	Assets  []Asset
}

// Asset è un file allegato alla release.
type Asset struct {
	Name string
	URL  string
	Size int64
	// Digest è l'impronta calcolata da GitHub ("sha256:<hex>"), vuota per le
	// release più vecchie: se c'è, il download viene verificato.
	Digest string
}

// Asset restituisce l'allegato con il nome indicato.
func (r Release) Asset(name string) (Asset, bool) {
	for _, a := range r.Assets {
		if strings.EqualFold(a.Name, name) {
			return a, true
		}
	}
	return Asset{}, false
}

func newRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// L'API di GitHub rifiuta le richieste senza User-Agent.
	req.Header.Set("User-Agent", "RenameMusic/"+Version)
	return req, nil
}

// Latest interroga GitHub sull'ultima release pubblicata. Restituisce
// ErrNoRelease se il repository non ne ha ancora.
func Latest(ctx context.Context) (Release, error) {
	req, err := newRequest(ctx, latestURL)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Release{}, ErrNoRelease
	}
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("GitHub ha risposto HTTP %d", resp.StatusCode)
	}

	var body struct {
		TagName string `json:"tag_name"`
		Body    string `json:"body"`
		Assets  []struct {
			Name   string `json:"name"`
			URL    string `json:"browser_download_url"`
			Size   int64  `json:"size"`
			Digest string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Release{}, fmt.Errorf("risposta di GitHub non valida: %w", err)
	}

	rel := Release{
		Version: strings.TrimPrefix(strings.TrimSpace(body.TagName), "v"),
		Notes:   strings.TrimSpace(body.Body),
	}
	for _, a := range body.Assets {
		rel.Assets = append(rel.Assets, Asset{Name: a.Name, URL: a.URL, Size: a.Size, Digest: a.Digest})
	}
	return rel, nil
}

// parseVersion scompone "1.2.3" (anche con "v" iniziale o suffissi come
// "-beta", ignorati) nei suoi numeri. ok=false se non è una versione.
func parseVersion(v string) (parts [3]int, ok bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+ "); i >= 0 {
		v = v[:i]
	}
	fields := strings.Split(v, ".")
	if len(fields) == 0 || len(fields) > 3 {
		return parts, false
	}
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil || n < 0 {
			return parts, false
		}
		parts[i] = n
	}
	return parts, true
}

// Newer indica se la versione `latest` è successiva a `current`. Una versione
// non interpretabile non è mai considerata più recente.
func Newer(latest, current string) bool {
	l, ok1 := parseVersion(latest)
	c, ok2 := parseVersion(current)
	if !ok1 || !ok2 {
		return false
	}
	for i := range l {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

// Download scarica l'allegato in dest (creando le cartelle mancanti), con
// l'avanzamento su progress (può essere nil). Scrive su un file temporaneo
// accanto a dest e lo rinomina solo dopo aver verificato dimensione e impronta
// sha256 (se GitHub la fornisce): un download interrotto o corrotto non lascia
// mai un file incompleto al posto giusto.
func Download(ctx context.Context, asset Asset, dest string, progress playlist.Progress) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	req, err := newRequest(ctx, asset.URL)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download fallito: HTTP %d", resp.StatusCode)
	}

	total := resp.ContentLength
	if total <= 0 {
		total = asset.Size
	}

	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	hash := sha256.New()
	pw := playlist.NewProgressWriter(progress, playlist.PhaseDownload, total)
	n, copyErr := io.Copy(io.MultiWriter(f, hash), io.TeeReader(resp.Body, pw))
	closeErr := f.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		os.Remove(tmp)
		return err
	}

	if asset.Size > 0 && n != asset.Size {
		os.Remove(tmp)
		return fmt.Errorf("download incompleto (%d di %d byte)", n, asset.Size)
	}
	if want, ok := strings.CutPrefix(asset.Digest, "sha256:"); ok {
		if got := hex.EncodeToString(hash.Sum(nil)); !strings.EqualFold(got, want) {
			os.Remove(tmp)
			return errors.New("il file scaricato è corrotto (impronta sha256 diversa)")
		}
	}

	os.Remove(dest)
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
