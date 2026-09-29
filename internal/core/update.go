package core

import (
	"context"
	"errors"
	"time"

	"renamemusic/internal/update"
)

// EventUpdateAvailable è emesso quando un controllo in background trova una
// nuova versione dell'app (diversa da quella eventualmente già nota). Il
// payload è l'UpdateView: la UI mostra il popup se Seen è false.
const EventUpdateAvailable = "update:available"

// UpdateView descrive la nuova versione disponibile. Seen indica che il popup
// per questa versione è già stato mostrato (vedi MarkUpdateSeen): la UI lo
// mostra una sola volta per versione, poi l'aggiornamento resta disponibile
// dalle Impostazioni.
type UpdateView struct {
	Version string `json:"version"`
	Notes   string `json:"notes"`
	Size    int64  `json:"size"`
	Seen    bool   `json:"seen"`
}

const (
	// updateRecheckInterval: dopo un controllo riuscito, il prossimo avviene
	// dopo qualche ora (l'app può restare aperta a lungo).
	updateRecheckInterval = 6 * time.Hour
	// updateRetryInterval: se il controllo fallisce (tipicamente: nessuna
	// connessione) si riprova spesso, così l'avviso arriva appena si è online.
	// Senza rete la richiesta fallisce subito, senza costi.
	updateRetryInterval = 2 * time.Minute
	// updateCheckTimeout limita la durata di una singola interrogazione a GitHub.
	updateCheckTimeout = 20 * time.Second
)

// updateLoop controlla periodicamente se c'è una nuova versione. Gira per
// tutta la vita dell'app, avviato da Start solo se la piattaforma fornisce un
// Updater.
func (a *App) updateLoop() {
	_ = a.safely("pulizia aggiornamenti", func() error {
		a.updater.Cleanup()
		return nil
	})
	for {
		wait := updateRecheckInterval
		err := a.safely("controllo aggiornamenti", func() error {
			_, err := a.checkUpdate()
			return err
		})
		if err != nil {
			wait = updateRetryInterval
		}
		time.Sleep(wait)
	}
}

// checkUpdate interroga GitHub sull'ultima release e aggiorna latestRelease.
// Se trova una versione nuova (non ancora nota) la notifica alla UI con
// EventUpdateAvailable. Restituisce la versione disponibile (nil se l'app è
// aggiornata o la release non ha un file per questa piattaforma).
func (a *App) checkUpdate() (*UpdateView, error) {
	a.checkMu.Lock()
	defer a.checkMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
	defer cancel()
	rel, err := update.Latest(ctx)
	if err != nil && !errors.Is(err, update.ErrNoRelease) {
		return nil, err
	}

	a.mu.Lock()
	prev := ""
	if a.latestRelease != nil {
		prev = a.latestRelease.Version
	}
	a.latestRelease = nil
	if err == nil && update.Newer(rel.Version, update.Version) {
		if asset, ok := rel.Asset(a.updater.Asset()); ok {
			a.latestRelease = &rel
			a.latestAsset = asset
		}
	}
	view := a.updateViewLocked()
	a.mu.Unlock()

	if view != nil && view.Version != prev {
		a.emit(EventUpdateAvailable, view)
	}
	return view, nil
}

// updateViewLocked costruisce l'UpdateView della versione disponibile (nil se
// non ce n'è). Va chiamata con il lock acquisito.
func (a *App) updateViewLocked() *UpdateView {
	if a.latestRelease == nil {
		return nil
	}
	return &UpdateView{
		Version: a.latestRelease.Version,
		Notes:   a.latestRelease.Notes,
		Size:    a.latestAsset.Size,
		Seen:    a.updateSeen == a.latestRelease.Version,
	}
}

// CheckUpdate controlla subito se c'è una nuova versione (tasto "Verifica
// aggiornamenti" nelle Impostazioni, o ritorno della connessione).
func (a *App) CheckUpdate() ActionResponse {
	if a.updater == nil {
		return ActionResponse{OK: false, Message: "Aggiornamenti non disponibili su questa piattaforma.", State: a.snapshotLocked()}
	}
	view, err := a.checkUpdate()
	if err != nil {
		return ActionResponse{OK: false, Message: "Impossibile verificare gli aggiornamenti: sei connesso a Internet?", State: a.snapshotLocked()}
	}
	if view == nil {
		return ActionResponse{OK: true, Message: "Hai già l'ultima versione (" + update.Version + ").", State: a.snapshotLocked()}
	}
	return ActionResponse{OK: true, Message: "È disponibile la versione " + view.Version + ".", State: a.snapshotLocked()}
}

// MarkUpdateSeen registra (e persiste) che il popup della versione indicata è
// stato mostrato, così non ricompare ai prossimi avvii.
func (a *App) MarkUpdateSeen(version string) ActionResponse {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.updateSeen != version {
		a.updateSeen = version
		a.persistStateLocked()
	}
	return ActionResponse{OK: true, State: a.snapshot()}
}

// InstallUpdate scarica la nuova versione (con l'avanzamento su
// EventInstallProgress, tool "RenameMusic") e la installa tramite l'Updater di
// piattaforma: su desktop sostituisce l'eseguibile e riavvia l'app, su Android
// apre l'installer di sistema.
func (a *App) InstallUpdate() ActionResponse {
	if a.updater == nil {
		return ActionResponse{OK: false, Message: "Aggiornamenti non disponibili su questa piattaforma.", State: a.snapshotLocked()}
	}

	a.mu.Lock()
	if a.latestRelease == nil {
		defer a.mu.Unlock()
		return ActionResponse{OK: false, Message: "Nessun aggiornamento disponibile.", State: a.snapshot()}
	}
	if a.updateInstalling {
		defer a.mu.Unlock()
		return ActionResponse{OK: false, Message: "Aggiornamento già in corso.", State: a.snapshot()}
	}
	a.updateInstalling = true
	version := a.latestRelease.Version
	asset := a.latestAsset
	a.addLogLocked(LogInfo, "Download della versione "+version+" in corso...")
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.updateInstalling = false
		a.mu.Unlock()
	}()

	fail := func(err error) ActionResponse {
		msg := "Aggiornamento non riuscito: " + err.Error()
		a.mu.Lock()
		defer a.mu.Unlock()
		a.addLogLocked(LogError, msg)
		return ActionResponse{OK: false, Message: msg, State: a.snapshot()}
	}

	dest, err := a.updater.DownloadPath()
	if err != nil {
		return fail(err)
	}
	if err := update.Download(context.Background(), asset, dest, a.installProgress("RenameMusic")); err != nil {
		return fail(err)
	}
	msg, err := a.updater.Apply(dest)
	if err != nil {
		return fail(err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.addLogLocked(LogSuccess, "Versione "+version+" scaricata. "+msg)
	return ActionResponse{OK: true, Message: msg, State: a.snapshot()}
}
