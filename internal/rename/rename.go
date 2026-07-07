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
					return results, err
				}
				result.Reason = "nome già esistente (originale eliminato)"
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
				return results, err
			}
		default:
			// Copia lasciando l'originale, sovrascrivendo un eventuale file di destinazione.
			if err := copyFile(it.path, it.newPath); err != nil {
				return results, err
			}
		}

		if it.ext == "mp3" {
			if err := WriteTags(it.newPath); err != nil {
				return results, err
			}
			result.Tagged = true
		}
		results = append(results, result)
	}
	return results, nil
}

func WriteTags(path string) error {
	name := filepath.Base(path)
	if parser.Extension(name) != "mp3" {
		return nil
	}
	return tags.WriteMP3Tags(path, parser.TagTitle(name), parser.TagArtist(name))
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

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
