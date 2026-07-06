package fs

import (
	"os"
	"path/filepath"

	"renamemusic/internal/parser"
	"renamemusic/internal/rules"
)

func ScanAudioFiles(folder string) ([]string, error) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := parser.Extension(entry.Name())
		if rules.IsSupportedExtension(ext) {
			files = append(files, filepath.Join(folder, entry.Name()))
		}
	}
	return files, nil
}

func IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
