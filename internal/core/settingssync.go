package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"renamemusic/internal/drive"
	"renamemusic/internal/google"
	"renamemusic/internal/playlist"
	"renamemusic/internal/rules"
	"renamemusic/internal/settings"
)

// settingsDriveFile è il file, nella cartella dati dell'app su Google Drive,
// con le impostazioni condivise fra i dispositivi collegati allo stesso
// account. Solo con l'account collegato: scollegandolo ogni dispositivo
// prosegue con i propri file locali, come prima.
const settingsDriveFile = "settings.json"

// Sezioni delle impostazioni sincronizzate. Restano invece del singolo
// dispositivo cartelle, opzioni di conversione, aggiornamento automatico e
// yt-dlp (state.json): sono percorsi e scelte legati alla macchina.
const (
	syncConfig           = "config"           // regole correnti (con la modalità semplificata)
	syncDefaults         = "defaults"         // regole predefinite
	syncPlaylists        = "playlists"        // playlist salvate
	syncDefaultPlaylists = "defaultPlaylists" // playlist predefinite
	syncPrefs            = "playlistPrefs"    // impostazioni delle singole playlist
)

var syncSections = []string{syncConfig, syncDefaults, syncPlaylists, syncDefaultPlaylists, syncPrefs}

// syncSection è una sezione di settingsDriveFile: il contenuto e l'istante
// dell'ultima modifica, con cui si sceglie la versione più recente.
type syncSection struct {
	Updated time.Time       `json:"updated"`
	Data    json.RawMessage `json:"data"`
}

// syncedSettings è il contenuto di settingsDriveFile.
type syncedSettings struct {
	Version  int                    `json:"version"`
	Sections map[string]syncSection `json:"sections"`
}

func (a *App) driveClient() *drive.Client {
	return &drive.Client{Token: a.google.Token, Invalidate: a.google.Invalidate}
}

// markChangedLocked segna le sezioni come modificate su questo dispositivo, da
// caricare sull'account. Va chiamata con il lock acquisito (le modifiche fatte
// senza account collegato si caricano al primo collegamento, se più recenti).
func (a *App) markChangedLocked(sections ...string) {
	if a.syncUpdated == nil {
		a.syncUpdated = map[string]time.Time{}
	}
	if a.syncDirty == nil {
		a.syncDirty = map[string]bool{}
	}
	now := time.Now().UTC()
	for _, s := range sections {
		a.syncUpdated[s] = now
		a.syncDirty[s] = true
	}
	a.persistStateLocked()
}

// localSectionLocked restituisce il contenuto attuale della sezione s, senza
// la cartella di partenza (che è del dispositivo). Va chiamata con il lock.
func (a *App) localSectionLocked(s string) (json.RawMessage, error) {
	var v any
	switch s {
	case syncConfig:
		c := a.config
		c.StartFolder = ""
		v = c
	case syncDefaults:
		c := a.defaults
		c.StartFolder = ""
		v = c
	case syncPlaylists:
		v = cleanPlaylists(a.playlists)
	case syncDefaultPlaylists:
		v = cleanPlaylists(a.defaultPlaylists)
	case syncPrefs:
		p := a.playlistPrefs
		if p == nil {
			p = map[string]playlist.Prefs{}
		}
		v = p
	default:
		return nil, fmt.Errorf("sezione sconosciuta %q", s)
	}
	return json.Marshal(v)
}

// applySection applica e salva la sezione s ricevuta dall'account. Fa I/O:
// va chiamata SENZA lock.
func (a *App) applySection(s string, data json.RawMessage) error {
	switch s {
	case syncConfig, syncDefaults:
		// Campi mancanti: quelli di fabbrica, come nei file locali.
		cfg := rules.FactoryConfig()
		if err := json.Unmarshal(data, &cfg); err != nil {
			return err
		}
		cfg = normalizeConfig(cfg)
		if s == syncConfig {
			_, err := a.storeConfig(cfg, false)
			return err
		}
		a.mu.Lock()
		cfg.StartFolder = a.config.StartFolder
		a.mu.Unlock()
		err := settings.SaveDefaults(cfg)
		a.mu.Lock()
		a.defaults = cfg
		a.mu.Unlock()
		return err
	case syncPlaylists, syncDefaultPlaylists:
		var list []playlist.Playlist
		if err := json.Unmarshal(data, &list); err != nil {
			return err
		}
		list = cleanPlaylists(list)
		save := settings.SavePlaylists
		if s == syncDefaultPlaylists {
			save = settings.SaveDefaultPlaylists
		}
		err := save(list)
		a.mu.Lock()
		if s == syncPlaylists {
			a.playlists = list
		} else {
			a.defaultPlaylists = list
		}
		a.mu.Unlock()
		return err
	case syncPrefs:
		prefs := map[string]playlist.Prefs{}
		if err := json.Unmarshal(data, &prefs); err != nil {
			return err
		}
		err := settings.SavePlaylistPrefs(prefs)
		a.mu.Lock()
		a.playlistPrefs = prefs
		a.mu.Unlock()
		return err
	}
	return fmt.Errorf("sezione sconosciuta %q", s)
}

// sameJSON confronta due contenuti JSON ignorando la formattazione.
func sameJSON(x, y []byte) bool {
	var cx, cy bytes.Buffer
	if json.Compact(&cx, x) != nil || json.Compact(&cy, y) != nil {
		return false
	}
	return bytes.Equal(cx.Bytes(), cy.Bytes())
}

// syncSettings allinea le impostazioni con quelle salvate sull'account
// Google, sezione per sezione: quelle dell'account più recenti (cambiate da
// un altro dispositivo) sostituiscono le locali; le locali più recenti, o non
// ancora caricate, si caricano. Restituisce true se ha cambiato qualcosa qui.
// Un errore resta in GoogleView.SyncError fino alla prossima sincronizzazione
// riuscita. Senza account collegato non fa nulla. Fa I/O di rete: va chiamata
// SENZA lock.
func (a *App) syncSettings(ctx context.Context) (changed bool) {
	if a.googleReady() != "" {
		return false
	}
	a.syncMu.Lock()
	defer a.syncMu.Unlock()

	c := a.driveClient()
	data, found, err := c.ReadAppFile(ctx, settingsDriveFile)
	if err != nil {
		a.syncFailed(err)
		return false
	}
	var remote syncedSettings
	if found && json.Unmarshal(data, &remote) != nil {
		remote = syncedSettings{} // file illeggibile: lo si riscrive con le impostazioni locali
	}

	type localSection struct {
		updated time.Time
		dirty   bool
		data    json.RawMessage
	}
	local := map[string]localSection{}
	a.mu.Lock()
	for _, s := range syncSections {
		data, err := a.localSectionLocked(s)
		if err != nil {
			a.mu.Unlock()
			a.syncFailed(err)
			return false
		}
		local[s] = localSection{updated: a.syncUpdated[s], dirty: a.syncDirty[s], data: data}
	}
	a.mu.Unlock()

	now := time.Now().UTC()
	out := syncedSettings{Version: 1, Sections: map[string]syncSection{}}
	applied := map[string]time.Time{}
	pushed := map[string]time.Time{}
	for _, s := range syncSections {
		l := local[s]
		r, inRemote := remote.Sections[s]
		switch {
		case inRemote && r.Updated.After(l.updated):
			out.Sections[s] = r
			if !sameJSON(r.Data, l.data) {
				if err := a.applySection(s, r.Data); err != nil {
					a.mu.Lock()
					a.addLogLocked(LogError, fmt.Sprintf("Impostazioni ricevute dall'account non applicate (%s): %v", s, err))
					a.mu.Unlock()
					continue
				}
				changed = true
			}
			applied[s] = r.Updated
		case !inRemote || l.dirty || l.updated.After(r.Updated):
			up := l.updated
			if up.IsZero() {
				up = now
			}
			out.Sections[s] = syncSection{Updated: up, Data: l.data}
			pushed[s] = up
		default:
			out.Sections[s] = r
		}
	}

	if len(pushed) > 0 {
		payload, err := json.MarshalIndent(out, "", "  ")
		if err == nil {
			err = c.WriteAppFile(ctx, settingsDriveFile, payload)
		}
		if err != nil {
			pushed = nil // restano da caricare
			a.syncFailed(err)
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.syncUpdated == nil {
		a.syncUpdated = map[string]time.Time{}
	}
	if a.syncDirty == nil {
		a.syncDirty = map[string]bool{}
	}
	for s, up := range applied {
		a.syncUpdated[s] = up
		delete(a.syncDirty, s)
	}
	for s, up := range pushed {
		// Se nel frattempo è cambiata di nuovo, resta da caricare l'ultima versione.
		if a.syncUpdated[s].Equal(local[s].updated) {
			a.syncUpdated[s] = up
			delete(a.syncDirty, s)
		}
	}
	if pushed != nil {
		a.syncErr = ""
	}
	if changed {
		a.addLogLocked(LogInfo, "Impostazioni aggiornate con quelle dell'account Google.")
		a.syncRevision++
	}
	a.persistStateLocked()
	return changed
}

// syncFailed registra l'errore di una sincronizzazione: un accesso scaduto
// scollega l'account, gli altri errori restano in SyncError (e nel registro,
// la prima volta).
func (a *App) syncFailed(err error) {
	if errors.Is(err, google.ErrSignedOut) {
		a.googleError(err)
		return
	}
	msg := "Impostazioni non sincronizzate con l'account: " + capitalize(err.Error()) + "."
	if errors.Is(err, context.DeadlineExceeded) {
		msg = "Impostazioni non sincronizzate con l'account: Google Drive non ha risposto in tempo."
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.syncErr != msg {
		a.addLogLocked(LogError, msg)
	}
	a.syncErr = msg
}

// settingsSyncInterval: con l'app aperta e l'account collegato, ogni quanto
// si controllano le impostazioni sull'account, così le modifiche fatte su un
// altro dispositivo compaiono da sole. Una lettura su Drive costa poco (la
// quota è per minuto, non per giorno come per YouTube).
const settingsSyncInterval = 30 * time.Second

// settingsSyncLoop controlla periodicamente le impostazioni sull'account e
// notifica la UI (EventGoogleChanged) quando arrivano modifiche. Gira per
// tutta la vita dell'app, avviato da Start; senza account non fa nulla.
func (a *App) settingsSyncLoop() {
	for {
		time.Sleep(settingsSyncInterval)
		_ = a.safely("sincronizzazione delle impostazioni", func() error {
			ctx, cancel := context.WithTimeout(context.Background(), googleCallTimeout)
			defer cancel()
			if a.syncSettings(ctx) {
				a.emit(EventGoogleChanged, a.snapshotLocked())
			}
			return nil
		})
	}
}

// SyncSettings confronta subito le impostazioni con quelle sull'account
// Google (la UI la chiama all'apertura delle Impostazioni, per mostrare
// quelle cambiate da un altro dispositivo). Senza account collegato non fa
// nulla; un errore è in State.Google.SyncError.
func (a *App) SyncSettings() ActionResponse {
	ctx, cancel := context.WithTimeout(context.Background(), googleCallTimeout)
	defer cancel()
	a.syncSettings(ctx)
	return ActionResponse{OK: true, State: a.snapshotLocked()}
}

// syncSettingsInBackground carica sull'account le impostazioni appena
// cambiate, senza far attendere la UI, e le notifica l'esito
// (EventGoogleChanged).
func (a *App) syncSettingsInBackground() {
	if a.googleReady() != "" {
		return
	}
	a.syncWG.Add(1)
	go func() {
		defer a.syncWG.Done()
		_ = a.safely("sincronizzazione delle impostazioni", func() error {
			ctx, cancel := context.WithTimeout(context.Background(), googleCallTimeout)
			defer cancel()
			a.syncSettings(ctx)
			a.emit(EventGoogleChanged, a.snapshotLocked())
			return nil
		})
	}()
}
