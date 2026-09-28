package core

import (
	"errors"
	"time"
)

// Aggiornamento automatico di yt-dlp: YouTube cambia spesso le sue protezioni
// e una versione vecchia di qualche settimana smette di scaricare (tipicamente
// "HTTP Error 403: Forbidden"). Con la gestione automatica attiva l'app tiene
// aggiornata da sé la propria copia; con un percorso scelto a mano il file è
// dell'utente e non lo tocchiamo.
const (
	// ytDlpRecheckInterval: dopo un controllo riuscito il prossimo avviene il
	// giorno dopo (l'app può restare aperta a lungo).
	ytDlpRecheckInterval = 24 * time.Hour
	// ytDlpRetryInterval: se il controllo fallisce (nessuna connessione) o un
	// download sta usando yt-dlp, si riprova dopo un po'.
	ytDlpRetryInterval = 15 * time.Minute
)

// errYtDlpBusy: yt-dlp è in uso (download o installazione) e non si può
// sostituire ora.
var errYtDlpBusy = errors.New("yt-dlp in uso")

// ytDlpUpdateLoop controlla periodicamente se c'è una nuova versione di yt-dlp
// e la installa. Gira per tutta la vita dell'app, avviato da Start. Un segnale
// su ytDlpCheck anticipa il controllo (yt-dlp appena inizializzato su Android,
// gestione automatica appena attivata).
func (a *App) ytDlpUpdateLoop() {
	for {
		wait := ytDlpRecheckInterval
		if err := a.autoUpdateYtDlp(); err != nil {
			wait = ytDlpRetryInterval
		}
		select {
		case <-time.After(wait):
		case <-a.ytDlpCheck:
		}
	}
}

// requestYtDlpCheck anticipa il prossimo controllo di ytDlpUpdateLoop. Non
// blocca: se un controllo è già stato richiesto non fa nulla.
func (a *App) requestYtDlpCheck() {
	select {
	case a.ytDlpCheck <- struct{}{}:
	default:
	}
}

// autoUpdateYtDlp installa la nuova versione di yt-dlp, se c'è. Non fa nulla
// (senza errore) se la gestione automatica è disattivata o yt-dlp non è ancora
// installato: l'installazione scarica comunque l'ultima versione. Gli errori di
// rete non finiscono nel registro Attività, per non riempirlo quando si è
// offline: si riprova e basta.
func (a *App) autoUpdateYtDlp() error {
	a.mu.Lock()
	managed, available := a.ytDlpManaged, a.ytDlpAvailable
	path := a.ytDlpEffectivePath()
	a.mu.Unlock()
	if !managed || !available || path == "" {
		return nil
	}

	if !a.ytDlpMu.TryLock() {
		return errYtDlpBusy
	}
	updated, err := a.yt().Update(path)
	a.ytDlpMu.Unlock()
	if err != nil || !updated {
		return err
	}

	a.mu.Lock()
	a.refreshYtDlpStatus()
	msg := "yt-dlp aggiornato automaticamente."
	if a.ytDlpVersion != "" {
		msg = "yt-dlp aggiornato automaticamente alla versione " + a.ytDlpVersion + "."
	}
	a.addLogLocked(LogSuccess, msg)
	state := a.snapshot()
	a.mu.Unlock()
	a.emit(EventYtDlpChanged, state)
	return nil
}
