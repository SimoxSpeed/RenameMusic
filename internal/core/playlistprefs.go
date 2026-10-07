package core

import (
	"maps"
	"strings"

	"renamemusic/internal/playlist"
	"renamemusic/internal/settings"
	"renamemusic/internal/youtube"
)

// googleKey è la chiave delle impostazioni della playlist di YouTube id.
func googleKey(id string) string { return "yt:" + id }

// playlistKey è la chiave delle impostazioni di una playlist salvata: quella
// della playlist di YouTube del link (così coincide con la stessa playlist
// dell'account), oppure il nome se il link non ha un ID di playlist.
func playlistKey(url, name string) string {
	if id := youtube.PlaylistID(url); id != "" {
		return googleKey(id)
	}
	return "pl:" + strings.TrimSpace(name)
}

// SetPlaylistPrefs salva le impostazioni della playlist con chiave key
// ("yt:<ID>" o "pl:<nome>", vedi StateResponse.PlaylistPrefs). Impostazioni
// tutte predefinite tolgono la voce. Come SetPlaylists, logga solo gli errori.
func (a *App) SetPlaylistPrefs(key string, prefs playlist.Prefs) ActionResponse {
	fail := func(msg string) ActionResponse {
		return ActionResponse{OK: false, Message: msg, State: a.snapshotLocked()}
	}
	if !strings.HasPrefix(key, "yt:") && !strings.HasPrefix(key, "pl:") || len(key) <= 3 {
		return fail("Playlist non valida.")
	}
	switch prefs.AfterDownload {
	case playlist.AfterNothing, playlist.AfterRemove:
		prefs.CopyTo, prefs.CopyToTitle, prefs.MoveOnCopy = "", "", false
	case playlist.AfterCopy:
		if prefs.CopyTo == "" {
			return fail("Scegli la playlist in cui aggiungere i brani scaricati.")
		}
		if googleKey(prefs.CopyTo) == key {
			return fail("I brani non si possono aggiungere alla stessa playlist da cui si scaricano.")
		}
	default:
		return fail("Azione dopo il download non valida.")
	}
	if (prefs.AfterDownload == playlist.AfterRemove || prefs.MoveOnCopy) && !strings.HasPrefix(key, "yt:") {
		return fail("Solo una playlist di YouTube del tuo account si può svuotare.")
	}

	a.mu.Lock()
	next := maps.Clone(a.playlistPrefs)
	if next == nil {
		next = map[string]playlist.Prefs{}
	}
	if prefs.IsZero() {
		delete(next, key)
	} else {
		next[key] = prefs
	}
	a.mu.Unlock()

	err := settings.SavePlaylistPrefs(next)

	a.mu.Lock()
	a.playlistPrefs = next
	// Da caricare sull'account Google, per gli altri dispositivi.
	a.markChangedLocked(syncPrefs)
	if err != nil {
		a.addLogLocked(LogError, "Impostazioni della playlist applicate ma NON salvate: "+err.Error())
	}
	state := a.snapshot()
	a.mu.Unlock()

	a.syncSettingsInBackground()
	if err != nil {
		return ActionResponse{OK: false, Message: "Impostazioni della playlist non salvate su disco.", State: state}
	}
	return ActionResponse{OK: true, State: state}
}
