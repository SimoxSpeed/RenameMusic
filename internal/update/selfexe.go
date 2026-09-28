package update

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Aggiornamento dell'eseguibile desktop. Su Windows il file di un programma in
// esecuzione non si può sovrascrivere né cancellare, ma si può rinominare: la
// nuova versione viene scaricata accanto all'eseguibile (NewSuffix), quello in
// uso viene spostato su OldSuffix e la nuova versione prende il suo nome. Al
// riavvio CleanupExecutable rimuove i residui.
const (
	NewSuffix = ".new"
	OldSuffix = ".old"
)

// Executable restituisce il percorso reale dell'eseguibile in esecuzione.
func Executable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// ReplaceExecutable sostituisce exe con il file newPath (che deve stare nella
// stessa cartella, per un semplice rename). In caso di errore a metà rimette a
// posto l'eseguibile originale.
func ReplaceExecutable(exe, newPath string) error {
	old := exe + OldSuffix
	_ = os.Remove(old) // residuo di un aggiornamento precedente, se c'è
	if err := os.Rename(exe, old); err != nil {
		return fmt.Errorf("impossibile sostituire l'eseguibile: %w", err)
	}
	if err := os.Rename(newPath, exe); err != nil {
		_ = os.Rename(old, exe)
		return fmt.Errorf("impossibile installare la nuova versione: %w", err)
	}
	return nil
}

// CleanupExecutable rimuove i residui di un aggiornamento accanto a exe: la
// versione precedente (che subito dopo il riavvio può essere ancora in chiusura,
// quindi si riprova per qualche secondo) e un eventuale download interrotto.
func CleanupExecutable(exe string) {
	_ = os.Remove(exe + NewSuffix)
	_ = os.Remove(exe + NewSuffix + ".part")
	old := exe + OldSuffix
	for i := 0; i < 10; i++ {
		if err := os.Remove(old); err == nil || os.IsNotExist(err) {
			return
		}
		time.Sleep(time.Second)
	}
}
