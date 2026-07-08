package rename

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"renamemusic/internal/rules"
)

func TestRenameAllRenamesOnDiskAndTagsMP3(t *testing.T) {
	dir := t.TempDir()

	// File non-MP3: deve solo essere rinominato secondo le regole.
	flacOld := filepath.Join(dir, "Artist X Guest - Song (Official Video).flac")
	if err := os.WriteFile(flacOld, []byte("dummy-flac"), 0o644); err != nil {
		t.Fatal(err)
	}
	// File MP3: deve essere rinominato E ricevere i tag ID3.
	mp3Old := filepath.Join(dir, "Producer - Track feat. Guest.mp3")
	if err := os.WriteFile(mp3Old, []byte("dummy-mp3-audio-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := rules.FactoryConfig()
	cfg.StartFolder = dir

	svc := NewService(cfg)
	files, err := svc.Scan()
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("Scan trovati %d file, attesi 2", len(files))
	}

	results, err := svc.Process(files, Options{DeleteOriginals: true})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	flacNew := filepath.Join(dir, "Artist x Guest - Song.flac")
	if _, err := os.Stat(flacNew); err != nil {
		t.Fatalf("file flac non rinominato correttamente: %v", err)
	}

	mp3New := filepath.Join(dir, "Producer - Track ft Guest.mp3")
	data, err := os.ReadFile(mp3New)
	if err != nil {
		t.Fatalf("file mp3 non rinominato correttamente: %v", err)
	}
	if len(data) < 3 || string(data[0:3]) != "ID3" {
		t.Fatalf("il file mp3 non ha un header ID3 scritto")
	}

	tagged := false
	for _, r := range results {
		if filepath.Base(r.NewPath) == "Producer - Track ft Guest.mp3" && r.Tagged {
			tagged = true
		}
	}
	if !tagged {
		t.Fatalf("il risultato MP3 non risulta taggato")
	}
}

func TestProcessSkipsBatchCollisionAndDeletesOriginal(t *testing.T) {
	dir := t.TempDir()

	// Due file che normalizzano allo stesso nome "Song.mp3".
	a := filepath.Join(dir, "Song (Lyrics).mp3")
	b := filepath.Join(dir, "Song (Official Video).mp3")
	if err := os.WriteFile(a, []byte("aaa"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("bbb"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := rules.FactoryConfig()
	cfg.StartFolder = dir

	svc := NewService(cfg)
	files, err := svc.Scan()
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	results, err := svc.Process(files, Options{DeleteOriginals: true})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	// Un solo "Song.mp3" deve esistere; entrambi gli originali spariti.
	if _, err := os.Stat(filepath.Join(dir, "Song.mp3")); err != nil {
		t.Fatalf("Song.mp3 mancante: %v", err)
	}
	if _, err := os.Stat(a); err == nil {
		t.Fatal("l'originale collidente doveva essere eliminato")
	}
	if _, err := os.Stat(b); err == nil {
		t.Fatal("l'originale collidente doveva essere eliminato")
	}

	skipped := 0
	for _, r := range results {
		if r.Skipped {
			skipped++
		}
	}
	if skipped != 1 {
		t.Fatalf("atteso 1 file saltato, ottenuti %d", skipped)
	}
}

func TestProcessKeepsAlreadyNamedFileOnCollision(t *testing.T) {
	dir := t.TempDir()

	// "Song.mp3" è già col nome finale; "Song (Official Video).mp3" ci collide.
	keep := filepath.Join(dir, "Song.mp3")
	junk := filepath.Join(dir, "Song (Official Video).mp3")
	if err := os.WriteFile(keep, []byte("KEEP_AUDIO"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(junk, []byte("JUNK_AUDIO"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := rules.FactoryConfig()
	cfg.StartFolder = dir

	svc := NewService(cfg)
	files, err := svc.Scan()
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	results, err := svc.Process(files, Options{DeleteOriginals: true})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	// Il file collidente deve essere stato eliminato e segnalato come saltato.
	if _, err := os.Stat(junk); err == nil {
		t.Fatal("il file collidente doveva essere eliminato")
	}
	skipped := 0
	for _, r := range results {
		if r.Skipped {
			skipped++
		}
	}
	if skipped != 1 {
		t.Fatalf("atteso 1 file saltato, ottenuti %d", skipped)
	}

	// Il file già col nome giusto NON deve essere stato sovrascritto dall'altro.
	data, err := os.ReadFile(keep)
	if err != nil {
		t.Fatalf("Song.mp3 mancante: %v", err)
	}
	if !strings.Contains(string(data), "KEEP_AUDIO") {
		t.Fatal("il file corretto è stato sovrascritto dal collidente (perdita di dati)")
	}
	if strings.Contains(string(data), "JUNK_AUDIO") {
		t.Fatal("il contenuto del file collidente ha sovrascritto quello corretto")
	}
}

func TestProcessContinuesAfterSingleFileFailure(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Due file distinti da copiare in destinazione.
	bad := filepath.Join(src, "Bad Song (Official Video).flac")  // normalizza -> "Bad Song.flac"
	good := filepath.Join(src, "Good Song (Official Video).flac") // normalizza -> "Good Song.flac"
	if err := os.WriteFile(bad, []byte("bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(good, []byte("good"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Sabotiamo la destinazione del primo file: creiamo una CARTELLA NON VUOTA
	// dove dovrebbe finire "Bad Song.flac", così la copia non può sostituirla e
	// fallisce. È il modo portabile per forzare un errore su un solo elemento.
	badDest := filepath.Join(dst, "Bad Song.flac")
	if err := os.Mkdir(badDest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(badDest, "keep"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := rules.FactoryConfig()
	cfg.StartFolder = src

	svc := NewService(cfg)
	files, err := svc.Scan()
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	results, err := svc.Process(files, Options{DestinationFolder: dst, DeleteOriginals: false})
	if err != nil {
		t.Fatalf("Process non deve restituire errore di batch: %v", err)
	}

	// Il file "buono" deve essere stato copiato nonostante il fallimento dell'altro.
	if _, err := os.Stat(filepath.Join(dst, "Good Song.flac")); err != nil {
		t.Fatalf("il file valido doveva essere elaborato: %v", err)
	}

	var failed, ok int
	for _, r := range results {
		if r.Failed {
			failed++
			if r.Reason == "" {
				t.Fatal("un risultato fallito deve avere una Reason")
			}
		} else {
			ok++
		}
	}
	if failed != 1 {
		t.Fatalf("atteso 1 file fallito, ottenuti %d", failed)
	}
	if ok != 1 {
		t.Fatalf("atteso 1 file elaborato con successo, ottenuti %d", ok)
	}

	// Nessun file temporaneo residuo deve restare in destinazione.
	entries, _ := os.ReadDir(dst)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("residuo temporaneo non ripulito: %s", e.Name())
		}
	}
}

func TestProcessCopiesToDestinationKeepingOriginals(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	orig := filepath.Join(src, "Artist X Guest - Song (Official Video).flac")
	if err := os.WriteFile(orig, []byte("dummy"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := rules.FactoryConfig()
	cfg.StartFolder = src

	svc := NewService(cfg)
	files, err := svc.Scan()
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	if _, err := svc.Process(files, Options{DestinationFolder: dst, DeleteOriginals: false}); err != nil {
		t.Fatalf("Process: %v", err)
	}

	// L'originale deve restare intatto...
	if _, err := os.Stat(orig); err != nil {
		t.Fatalf("l'originale doveva restare: %v", err)
	}
	// ...e la copia normalizzata deve trovarsi nella destinazione.
	if _, err := os.Stat(filepath.Join(dst, "Artist x Guest - Song.flac")); err != nil {
		t.Fatalf("la copia in destinazione manca: %v", err)
	}
}
