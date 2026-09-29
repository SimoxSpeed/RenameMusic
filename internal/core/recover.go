package core

import "fmt"

// safely esegue fn recuperando un eventuale panic. Serve alle goroutine di
// background del core (controllo aggiornamenti, aggiornamento di yt-dlp,
// riscansione automatica): un panic non recuperato in una goroutine chiude
// l'intero processo, e su Android non passa da Call, che recupera solo quelli
// delle chiamate della UI. Il panic diventa un errore nel registro Attività.
func (a *App) safely(where string, fn func() error) (err error) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		err = fmt.Errorf("errore interno in %s: %v", where, r)
		// TryLock: se il panic è avvenuto con il lock acquisito, Lock
		// resterebbe bloccato per sempre.
		if a.mu.TryLock() {
			a.addLogLocked(LogError, "Errore interno ("+where+"): "+fmt.Sprint(r))
			a.mu.Unlock()
		}
	}()
	return fn()
}
