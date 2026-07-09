package rename

import (
	"io"
	"os"
	"path/filepath"

	"renamemusic/internal/fs"
	"renamemusic/internal/parser"
	"renamemusic/internal/rules"
	"renamemusic/internal/tags"
)

type Service struct {
	Config rules.Config
}

// Options controlla come vengono scritti i file elaborati.
type Options struct {
	DestinationFolder string // "" => stessa cartella di partenza
	DeleteOriginals   bool   // true => sposta/rinomina; false => scrive una copia lasciando gli originali
}

type Result struct {
	OldPath string
	NewPath string
	NewName string
	Tagged  bool
	Skipped bool
	Failed  bool // errore sul singolo file: il batch prosegue comunque
	Reason  string
}

func NewService(cfg rules.Config) *Service {
	return &Service{Config: cfg}
}

func (s *Service) Scan() ([]string, error) {
	return fs.ScanAudioFiles(s.Config.StartFolder, s.Config)
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

	// 3) Applica.
	results := make([]Result, 0, len(paths))
	for i := range items {
		it := items[i]
		if !it.support {
			continue
		}
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
			results = append(results, result)
			continue
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
				results = append(results, result)
				continue
			}
		default:
			// Copia lasciando l'originale, sovrascrivendo un eventuale file di destinazione.
			if err := copyFile(it.path, it.newPath); err != nil {
				result.Failed = true
				result.Reason = "copia fallita: " + err.Error()
				results = append(results, result)
				continue
			}
		}

		if it.ext == "mp3" {
			if err := s.WriteTags(it.newPath); err != nil {
				// Il file è stato rinominato/copiato, ma i tag non sono stati scritti.
				result.Failed = true
				result.Reason = "rinominato, ma scrittura tag fallita: " + err.Error()
				results = append(results, result)
				continue
			}
			result.Tagged = true
		}
		results = append(results, result)
	}
	return results, nil
}

func (s *Service) WriteTags(path string) error {
	name := filepath.Base(path)
	if parser.Extension(name) != "mp3" {
		return nil
	}
	exceptions := s.Config.ArtistExceptions
	return tags.WriteMP3Tags(path, parser.TagTitle(name, exceptions), parser.TagArtist(name, exceptions))
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

	tmp := dst + ".tmp"
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
