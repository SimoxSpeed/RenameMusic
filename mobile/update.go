package mobile

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"renamemusic/internal/settings"
)

// InstallPermission è la risposta di Host.InstallApk quando Android chiede
// prima il permesso di installare app da RenameMusic.
const InstallPermission = "permission"

// checkUpdates abilita il controllo delle nuove versioni; i test lo spengono
// per non interrogare GitHub.
var checkUpdates = true

// updateDir è la sottocartella della cartella privata dell'app in cui si
// scarica l'APK: la espone all'installer il FileProvider (res/xml/file_paths.xml).
const updateDir = "update"

// androidUpdater implementa core.Updater su Android: scarica l'APK dell'ABI
// del telefono e lo passa all'installer di sistema (Host.InstallApk). L'APK
// deve essere firmato con la stessa chiave di quello installato, altrimenti
// Android rifiuta l'aggiornamento.
type androidUpdater struct {
	host Host
}

// Asset è l'APK per l'architettura in uso, con i nomi prodotti da build-apk.ps1.
func (androidUpdater) Asset() string {
	if runtime.GOARCH == "amd64" {
		return "RenameMusic-x86_64.apk"
	}
	return "RenameMusic-arm64-v8a.apk"
}

func (u androidUpdater) DownloadPath() (string, error) {
	dir, err := settings.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, updateDir, u.Asset()), nil
}

func (u androidUpdater) Apply(path string) (string, error) {
	switch res := u.host.InstallApk(path); res {
	case "":
		return "Conferma l'installazione nella schermata di Android.", nil
	case InstallPermission:
		return "Consenti a RenameMusic di installare app, poi torna qui: l'installazione partirà da sola.", nil
	default:
		return "", errors.New(res)
	}
}

// Cleanup rimuove l'APK di un aggiornamento precedente (già installato o
// annullato).
func (androidUpdater) Cleanup() {
	if dir, err := settings.Dir(); err == nil {
		_ = os.RemoveAll(filepath.Join(dir, updateDir))
	}
}
