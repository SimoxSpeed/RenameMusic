package playlist

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ytDlpLatestURL è la pagina dell'ultima release di yt-dlp: GitHub risponde con
// un redirect a .../releases/tag/<versione>, da cui leggiamo la versione senza
// passare dall'API (che ha un limite di richieste orarie per IP).
const ytDlpLatestURL = "https://github.com/yt-dlp/yt-dlp/releases/latest"

// LatestVersion restituisce la versione dell'ultima release stabile di yt-dlp
// (es. "2026.08.19").
func LatestVersion() (string, error) {
	client := &http.Client{
		Timeout: 20 * time.Second,
		// Il redirect non va seguito: la versione è nella sua destinazione.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Get(ytDlpLatestURL)
	if err != nil {
		return "", fmt.Errorf("controllo della versione di yt-dlp fallito: %w", err)
	}
	resp.Body.Close()
	loc := resp.Header.Get("Location")
	i := strings.LastIndex(loc, "/tag/")
	if resp.StatusCode/100 != 3 || i < 0 {
		return "", fmt.Errorf("controllo della versione di yt-dlp fallito: HTTP %d", resp.StatusCode)
	}
	version := loc[i+len("/tag/"):]
	if _, ok := parseYtDlpVersion(version); !ok {
		return "", fmt.Errorf("versione di yt-dlp non riconosciuta: %q", version)
	}
	return version, nil
}

// NewerVersion indica se la versione di yt-dlp `latest` è successiva a
// `current`. Le versioni sono date con eventuali segmenti in più
// (es. "2026.08.19" o "2026.08.19.1"). Una versione corrente vuota o non
// riconoscibile conta come più vecchia (così una copia rovinata si reinstalla);
// una `latest` non riconoscibile non è mai più nuova.
func NewerVersion(latest, current string) bool {
	l, ok := parseYtDlpVersion(latest)
	if !ok {
		return false
	}
	c, ok := parseYtDlpVersion(current)
	if !ok {
		return true
	}
	for i := 0; i < len(l) && i < len(c); i++ {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return len(l) > len(c)
}

func parseYtDlpVersion(v string) ([]int, bool) {
	parts := strings.Split(strings.TrimSpace(v), ".")
	nums := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil, false
		}
		nums[i] = n
	}
	return nums, len(nums) >= 3
}
