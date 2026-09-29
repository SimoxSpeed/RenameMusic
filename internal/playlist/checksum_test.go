package playlist

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func TestParseSHA256Sums(t *testing.T) {
	sum := sha256Hex([]byte("x"))
	data := []byte("0000  altro.exe\n" + sum + "  yt-dlp.exe\n" + sha256Hex([]byte("y")) + " *ffmpeg.zip\n")

	if got, ok := parseSHA256Sums(data, "yt-dlp.exe"); !ok || got != sum {
		t.Errorf("yt-dlp.exe: got %q, %v", got, ok)
	}
	if got, ok := parseSHA256Sums(data, "ffmpeg.zip"); !ok || got != sha256Hex([]byte("y")) {
		t.Errorf("ffmpeg.zip (marcatore *): got %q, %v", got, ok)
	}
	if _, ok := parseSHA256Sums(data, "altro.exe"); ok {
		t.Error("altro.exe: un'impronta non valida non deve essere accettata")
	}
	if _, ok := parseSHA256Sums(data, "yt-dlp"); ok {
		t.Error("yt-dlp: il nome deve corrispondere esattamente")
	}
}

// serveRelease pubblica `files` (nome → contenuto) come una release, con il
// file di impronte `sumsName` generato da `sums` (nome → contenuto da cui
// calcolare l'impronta).
func serveRelease(t *testing.T, files map[string][]byte, sumsName string, sums func() map[string][]byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Base(r.URL.Path)
		if name == sumsName {
			var b bytes.Buffer
			for n, content := range sums() {
				b.WriteString(sha256Hex(content) + "  " + n + "\n")
			}
			w.Write(b.Bytes())
			return
		}
		content, ok := files[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(content)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestInstallYtDlpVerifiesChecksum(t *testing.T) {
	exe := []byte("yt-dlp nuovo")
	cases := []struct {
		name      string
		published []byte // contenuto da cui è calcolata l'impronta pubblicata
		wantErr   bool
	}{
		{"impronta corretta", exe, false},
		{"file corrotto", []byte("altro"), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := serveRelease(t, map[string][]byte{ytDlpAsset: exe}, ytDlpSumsAsset,
				func() map[string][]byte { return map[string][]byte{ytDlpAsset: c.published} })

			dest := filepath.Join(t.TempDir(), "yt-dlp.exe")
			if err := os.WriteFile(dest, []byte("yt-dlp vecchio"), 0o644); err != nil {
				t.Fatal(err)
			}
			err := installYtDlp(dest, srv.URL+"/"+ytDlpAsset, srv.URL+"/"+ytDlpSumsAsset, nil)

			got, _ := os.ReadFile(dest)
			if c.wantErr {
				if !errors.Is(err, errChecksumMismatch) {
					t.Fatalf("errore = %v, atteso errChecksumMismatch", err)
				}
				if string(got) != "yt-dlp vecchio" {
					t.Fatalf("con impronta diversa l'eseguibile esistente va lasciato com'è, trovato %q", got)
				}
			} else {
				if err != nil {
					t.Fatalf("installYtDlp: %v", err)
				}
				if !bytes.Equal(got, exe) {
					t.Fatalf("contenuto installato %q, atteso %q", got, exe)
				}
			}
			entries, _ := os.ReadDir(filepath.Dir(dest))
			if len(entries) != 1 {
				t.Fatalf("restano file temporanei: %v", entries)
			}
		})
	}
}

// ffmpegZip crea uno zip con la stessa struttura della build di FFmpeg-Builds.
func ffmpegZip(t *testing.T, content string) []byte {
	t.Helper()
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for _, name := range ffmpegBinaries {
		w, err := zw.Create("ffmpeg-master-latest-win64-gpl/bin/" + name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(content))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestInstallFFmpegVerifiesChecksum(t *testing.T) {
	zipData := ffmpegZip(t, "ffmpeg")
	other := ffmpegZip(t, "altro")

	cases := []struct {
		name    string
		sums    func(calls *int32) []byte // contenuto da cui calcolare l'impronta, alla n-esima lettura
		wantErr bool
	}{
		{"impronta corretta", func(*int32) []byte { return zipData }, false},
		{"file corrotto", func(*int32) []byte { return other }, true},
		{
			// La release è stata ripubblicata dopo la prima lettura delle
			// impronte: la seconda lettura corrisponde allo zip scaricato.
			"release ripubblicata durante il download",
			func(calls *int32) []byte {
				if atomic.AddInt32(calls, 1) == 1 {
					return other
				}
				return zipData
			},
			false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var calls int32
			srv := serveRelease(t, map[string][]byte{ffmpegAsset: zipData}, ffmpegSumsAsset,
				func() map[string][]byte { return map[string][]byte{ffmpegAsset: c.sums(&calls)} })

			dir := t.TempDir()
			err := installFFmpeg(dir, srv.URL+"/"+ffmpegAsset, srv.URL+"/"+ffmpegSumsAsset, nil)
			if c.wantErr {
				if !errors.Is(err, errChecksumMismatch) {
					t.Fatalf("errore = %v, atteso errChecksumMismatch", err)
				}
				if entries, _ := os.ReadDir(dir); len(entries) != 0 {
					t.Fatalf("con impronta diversa non va estratto nulla, trovati %v", entries)
				}
				return
			}
			if err != nil {
				t.Fatalf("installFFmpeg: %v", err)
			}
			if !FFmpegInstalled(dir) {
				t.Fatal("ffmpeg.exe/ffprobe.exe non estratti")
			}
		})
	}
}
