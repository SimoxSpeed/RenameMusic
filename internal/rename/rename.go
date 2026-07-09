package rename

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"renamemusic/internal/fs"
	"renamemusic/internal/parser"
	"renamemusic/internal/rules"
	"renamemusic/internal/tags"
)

// tempSuffix marca i file temporanei creati durante copyFile: un crash tra la
// scrittura del temp e il rename può lasciarli orfani. Il suffisso, volutamente
// specifico, consente a CleanTempFiles di rimuovere SOLO i nostri residui e mai
// un eventuale ".tmp" legittimo dell'utente nella cartella musicale.
const tempSuffix = ".renamemusic.tmp"

type Service struct {
	Config rules.Config
}

// Options controlla come vengono scritti i file elaborati.
type Options struct {
	DestinationFolder string // "" => stessa cartella di partenza
	DeleteOriginals   bool   // true => sposta/rinomina; false => scrive una copia lasciando gli originali

	// OnProgress, se valorizzata, viene invocata dopo l'elaborazione di ogni file
	// supportato con (done, total): done = file completati finora, total = numero
	// totale di file supportati nel batch. Serve a mostrare l'avanzamento in UI.
	OnProgress func(done, total int)

	// Cancelled, se valorizzata, viene interrogata prima di ogni file: se ritorna
	// true l'elaborazione si ferma (i file già elaborati restano nei Result).
	Cancelled func() bool
}

type Result struct {
	OldPath  string
	NewPath  string
	NewName  string
	Tagged   bool
	Skipped  bool
	Failed   bool // errore sul singolo file: il batch prosegue comunque
	Canceled bool // file non elaborato perché l'operazione è stata annullata
	Reason   string
}

func NewService(cfg rules.Config) *Service {
	return &Service{Config: cfg}
}

func (s *Service) Scan() ([]string, error) {
	return fs.ScanAudioFiles(s.Config.StartFolder, s.Config)
}

// CleanTempFiles rimuove dalla cartella indicata i file temporanei orfani
// lasciati da una copia interrotta (crash): agisce SOLO sui file col suffisso
// marcato (tempSuffix). Non ricorsivo. Restituisce quanti file ha rimosso;
// una cartella inesistente/illeggibile non è un errore fatale (removed=0).
func CleanTempFiles(folder string) int {
	if folder == "" {
		return 0
	}
	entries, err := os.ReadDir(folder)
	if err != nil {
		return 0
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), tempSuffix) {
			continue
		}
		if err := os.Remove(filepath.Join(folder, e.Name())); err == nil {
			removed++
		}
	}
	return removed
}

type workItem struct {
	path    string
	newName string
	newPath string
	ext     string
	support bool
	skip    bool
}

// Process normalizza e scrive ogni file secondo le Options fornite.
//
// Se più file del batch normalizzano allo stesso nome, solo uno "vince" e gli
// altri vengono saltati (ed eliminati se DeleteOriginals). A parità di nome viene
// preferito come vincitore il file che ha GIÀ il nome corretto, così non viene
// sovrascritto/cancellato per errore.
//
// Un errore su un singolo file NON interrompe il batch: il file viene marcato
// come Failed (con Reason) e l'elaborazione prosegue con i successivi. Così un
// disco pieno o un permesso mancante a metà lista non lascia il resto dei file
// non elaborato e senza feedback. L'error restituito resta nil (gli esiti per
// file sono nei Result); è mantenuto in firma per compatibilità/estensioni future.
func (s *Service) Process(paths []string, opts Options) ([]Result, error) {
	dest := opts.DestinationFolder
	if dest == "" {
		dest = s.Config.StartFolder
	}

	// 1) Calcola il nome di destinazione per ogni file supportato.
	items := make([]workItem, len(paths))
	for i, path := range paths {
		name := filepath.Base(path)
		ext := parser.Extension(name)
		if !s.Config.IsSupportedExtension(ext) {
			items[i] = workItem{path: path}
			continue
		}
		newName := s.Config.NormalizeFileBase(parser.RemoveExtension(name)) + "." + ext
		items[i] = workItem{
			path:    path,
			newName: newName,
			newPath: filepath.Join(dest, newName),
			ext:     ext,
			support: true,
		}
	}

	// 2) Determina il vincitore per ogni nome di destinazione (collisioni nel batch).
	winner := make(map[string]int)
	for i := range items {
		if !items[i].support {
			continue
		}
		key := filepath.Clean(items[i].newPath)
		samePath := key == filepath.Clean(items[i].path)
		if w, claimed := winner[key]; !claimed {
			winner[key] = i
		} else if samePath {
			// Questo file ha già il nome giusto: preferiamolo, l'altro salta.
			items[w].skip = true
			winner[key] = i
		} else {
			items[i].skip = true
		}
	}

	// 3) Applica. total = numero di file supportati (quelli che produrranno un
	// Result), usato per l'avanzamento.
	total := 0
	for i := range items {
		if items[i].support {
			total++
		}
	}

	results := make([]Result, 0, len(paths))
	done := 0
	canceled := false
	for i := range items {
		if !items[i].support {
			continue
		}
		if !canceled && opts.Cancelled != nil && opts.Cancelled() {
			canceled = true
		}
		if canceled {
			// Una volta annullato, i file supportati rimanenti non vengono
			// elaborati ma restano nei Result marcati come Canceled, così la UI
			// può mostrarli con stato "Annullato".
			results = append(results, Result{
				OldPath:  items[i].path,
				NewName:  items[i].newName,
				NewPath:  items[i].newPath,
				Canceled: true,
				Reason:   "operazione annullata",
			})
			continue
		}
		results = append(results, s.applyItem(items[i], opts))
		done++
		if opts.OnProgress != nil {
			opts.OnProgress(done, total)
		}
	}
	return results, nil
}

// applyItem esegue l'elaborazione di un singolo file supportato (spostamento o
// copia secondo le Options, più scrittura tag per gli MP3) e ne restituisce il
// Result. Un errore sul file NON interrompe il batch: viene riportato in Failed.
func (s *Service) applyItem(it workItem, opts Options) Result {
	result := Result{OldPath: it.path, NewName: it.newName, NewPath: it.newPath}

	if it.skip {
		result.Skipped = true
		if opts.DeleteOriginals {
			if err := os.Remove(it.path); err != nil {
				result.Failed = true
				result.Reason = "nome già esistente, ma eliminazione dell'originale fallita: " + err.Error()
			} else {
				result.Reason = "nome già esistente (originale eliminato)"
			}
		} else {
			result.Reason = "nome già esistente"
		}
		return result
	}

	samePath := filepath.Clean(it.newPath) == filepath.Clean(it.path)
	switch {
	case samePath:
		// Nome già corretto: nulla da spostare/copiare (i tag si scrivono comunque).
	case opts.DeleteOriginals:
		// Sposta/rinomina, sovrascrivendo un eventuale file preesistente con quel nome.
		if err := moveFile(it.path, it.newPath); err != nil {
			result.Failed = true
			result.Reason = "spostamento fallito: " + err.Error()
			return result
		}
	default:
		// Copia lasciando l'originale, sovrascrivendo un eventuale file di destinazione.
		if err := copyFile(it.path, it.newPath); err != nil {
			result.Failed = true
			result.Reason = "copia fallita: " + err.Error()
			return result
		}
	}

	if it.ext == "mp3" {
		if err := s.WriteTags(it.newPath); err != nil {
			// Il file è stato rinominato/copiato, ma i tag non sono stati scritti.
			result.Failed = true
			result.Reason = "rinominato, ma scrittura tag fallita: " + err.Error()
			return result
		}
		result.Tagged = true
	}
	return result
}

// ClearTags cancella i tag ID3 da tutti gli MP3 tra i percorsi indicati, in
// posto (senza rinominare). I file non-MP3 vengono ignorati. onProgress (se non
// nil) riceve (done, total) dopo ogni MP3; cancelled (se non nil) è interrogata
// prima di ogni file e, se true, ferma l'operazione. Restituisce quanti file
// sono stati ripuliti e quanti hanno fallito.
func (s *Service) ClearTags(paths []string, onProgress func(done, total int), cancelled func() bool) (cleared, failed int) {
	total := 0
	for _, p := range paths {
		if parser.Extension(filepath.Base(p)) == "mp3" {
			total++
		}
	}

	done := 0
	for _, p := range paths {
		if cancelled != nil && cancelled() {
			break
		}
		if parser.Extension(filepath.Base(p)) != "mp3" {
			continue
		}
		if err := tags.ClearMP3Tags(p); err != nil {
			failed++
		} else {
			cleared++
		}
		done++
		if onProgress != nil {
			onProgress(done, total)
		}
	}
	return cleared, failed
}

func (s *Service) WriteTags(path string) error {
	name := filepath.Base(path)
	if parser.Extension(name) != "mp3" {
		return nil
	}
	exceptions := s.Config.ArtistExceptions
	title := parser.TagTitle(name, exceptions)
	artist := parser.TagArtist(name, exceptions)

	if err := tags.WriteMP3Tags(path, title, artist); err != nil {
		return err
	}

	// Verifica round-trip: rileggiamo i tag appena scritti con un parser
	// indipendente. Se la rilettura fallisce o non corrisponde, la scrittura è
	// difettosa e il file va segnalato come non elaborato correttamente.
	gotTitle, gotArtist, err := tags.ReadMP3Tags(path)
	if err != nil {
		return fmt.Errorf("verifica tag non riuscita: %w", err)
	}
	if gotTitle != title || gotArtist != artist {
		return fmt.Errorf("tag riletti non corrispondono (titolo %q≠%q, artista %q≠%q)", gotTitle, title, gotArtist, artist)
	}
	return nil
}

// moveFile sposta src su dst sovrascrivendo un eventuale file esistente, con
// fallback copia+rimozione se il rename fallisce (es. cartelle su volumi diversi).
func moveFile(src, dst string) error {
	// os.Rename su Windows fallisce se dst esiste: rimuoviamo prima l'eventuale file.
	_ = os.Remove(dst)
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

// copyFile copia src su dst in modo atomico: scrive prima su un file temporaneo
// e lo rinomina su dst solo a copia completata. Così un errore a metà (es. disco
// pieno) non lascia un dst troncato/corrotto, e un eventuale dst preesistente non
// viene distrutto se la copia fallisce.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + tempSuffix
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}

	if err := os.Rename(tmp, dst); err != nil {
		// Alcuni casi (es. dst di sola lettura su Windows) non consentono la
		// sostituzione diretta: rimuoviamo dst e riproviamo. Il temp è già
		// completo, quindi la finestra di rischio è minima.
		_ = os.Remove(dst)
		if err2 := os.Rename(tmp, dst); err2 != nil {
			_ = os.Remove(tmp)
			return err2
		}
	}
	return nil
}
