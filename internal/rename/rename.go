package rename

import (
	"os"
	"path/filepath"

	"renamemusic/internal/fs"
	"renamemusic/internal/parser"
	"renamemusic/internal/rules"
	"renamemusic/internal/tags"
)

type Service struct {
	StartFolder string
}

type Result struct {
	OldPath string
	NewPath string
	NewName string
	Tagged  bool
}

func NewService(startFolder string) *Service {
	return &Service{StartFolder: startFolder}
}

func (s *Service) Scan() ([]string, error) {
	return fs.ScanAudioFiles(s.StartFolder)
}

func (s *Service) RenameFile(path string) (Result, error) {
	result := Result{OldPath: path}
	name := filepath.Base(path)
	ext := parser.Extension(name)
	if !rules.IsSupportedExtension(ext) {
		return result, nil
	}

	newName := rules.NormalizeFileBase(parser.RemoveExtension(name)) + "." + ext
	newPath := filepath.Join(s.StartFolder, newName)
	result.NewName = newName
	result.NewPath = newPath

	if path != newPath {
		if err := os.Rename(path, newPath); err != nil {
			return result, err
		}
	}

	if ext == "mp3" {
		if err := WriteTags(newPath); err != nil {
			return result, err
		}
		result.Tagged = true
	}

	return result, nil
}

func (s *Service) RenameAll(paths []string) ([]Result, error) {
	results := make([]Result, 0, len(paths))
	for _, path := range paths {
		result, err := s.RenameFile(path)
		if err != nil {
			return results, err
		}
		if result.NewName != "" {
			results = append(results, result)
		}
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

func WriteTagsAll(paths []string) (int, error) {
	written := 0
	for _, path := range paths {
		if parser.Extension(filepath.Base(path)) != "mp3" {
			continue
		}
		if err := WriteTags(path); err != nil {
			return written, err
		}
		written++
	}
	return written, nil
}
