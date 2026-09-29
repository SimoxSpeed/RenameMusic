package playlist

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// errChecksumMismatch indica un file scaricato con impronta diversa da quella
// pubblicata: download corrotto o manomesso.
var errChecksumMismatch = errors.New("il file scaricato è corrotto (impronta sha256 diversa)")

// fetchSHA256 scarica il file di impronte `sumsURL` (formato di sha256sum:
// "<hex>  <nome>", una riga per file) pubblicato accanto ai file di una
// release e restituisce l'impronta di `name`, in minuscolo.
func fetchSHA256(sumsURL, name string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(sumsURL)
	if err != nil {
		return "", fmt.Errorf("download delle impronte sha256 fallito: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download delle impronte sha256 fallito: HTTP %d", resp.StatusCode)
	}
	// Il file è di poche righe: 1 MB basta e avanza.
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("download delle impronte sha256 fallito: %w", err)
	}
	sum, ok := parseSHA256Sums(data, name)
	if !ok {
		return "", fmt.Errorf("impronta sha256 di %s non pubblicata", name)
	}
	return sum, nil
}

// parseSHA256Sums cerca in un file di impronte (formato di sha256sum) la riga
// di `name` e ne restituisce l'impronta in minuscolo. Accetta anche il
// marcatore "*" della modalità binaria davanti al nome.
func parseSHA256Sums(data []byte, name string) (string, bool) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != name {
			continue
		}
		sum := strings.ToLower(fields[0])
		if !isSHA256Hex(sum) {
			return "", false
		}
		return sum, true
	}
	return "", false
}

func isSHA256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
