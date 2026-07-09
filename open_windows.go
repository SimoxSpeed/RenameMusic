package main

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	modshell32       = syscall.NewLazyDLL("shell32.dll")
	procShellExecute = modshell32.NewProc("ShellExecuteW")

	modole32           = syscall.NewLazyDLL("ole32.dll")
	procCoInitializeEx = modole32.NewProc("CoInitializeEx")
	procCoUninitialize = modole32.NewProc("CoUninitialize")
)

const (
	swShowNormal           = 1
	coinitApartmentThread  = 0x2 // COINIT_APARTMENTTHREADED
	shellExecuteMinSuccess = 32  // ShellExecuteW: valore di ritorno > 32 = successo
)

// openFolderInShell apre `path` nella shell di Windows GIÀ in esecuzione tramite
// ShellExecuteW (verbo "open"), senza avviare un nuovo processo explorer.exe:
// l'apertura è quasi istantanea perché riusa l'Explorer attivo (a differenza di
// `explorer <path>`, che deve fare il cold-start di un nuovo processo).
//
// ShellExecuteW può delegare a estensioni della shell implementate come oggetti
// COM, quindi inizializziamo COM in modalità STA sul thread corrente (bloccato
// per la durata della chiamata) e la bilanciamo con CoUninitialize.
func openFolderInShell(path string) error {
	verb, err := syscall.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	target, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// S_OK (0) o S_FALSE (1): COM inizializzato da noi, va bilanciato con
	// CoUninitialize. Altri valori (es. RPC_E_CHANGED_MODE) => non uninizializzare.
	if hr, _, _ := procCoInitializeEx.Call(0, coinitApartmentThread); hr == 0 || hr == 1 {
		defer procCoUninitialize.Call()
	}

	ret, _, _ := procShellExecute.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(target)),
		0,
		0,
		uintptr(swShowNormal),
	)
	if ret <= shellExecuteMinSuccess {
		return fmt.Errorf("ShellExecuteW ha restituito %d", ret)
	}
	return nil
}
