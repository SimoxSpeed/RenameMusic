package core

import (
	"context"
	"path/filepath"
	"time"

	"renamemusic/internal/musicbrainz"
	"renamemusic/internal/parser"
)

// suggestTimeout limita una ricerca su MusicBrainz, attese del turno comprese
// (una richiesta al secondo, fino a quattro per traccia).
const suggestTimeout = 20 * time.Second

// TrackSuggestions è l'esito di SuggestTrackNames: Names sono i nomi base
// proposti (senza estensione, già normalizzati), dal più probabile; Error è il
// motivo per cui la ricerca non è riuscita (vuoto se è andata a buon fine,
// anche senza risultati).
type TrackSuggestions struct {
	Names []string `json:"names"`
	Error string   `json:"error,omitempty"`
}

// SuggestTrackNames cerca su MusicBrainz la traccia di una richiesta di
// conferma di ProcessAll (TagPromptView), per proporre nel popup un nome al
// posto di quello attuale. Per una traccia con tag sconosciuti cerca il nome
// normalizzato (musicbrainz.Lookup: con il solo titolo, i brani con quel
// titolo di artisti diversi); per una da rivedere, titolo e artista dedotti dal
// nome (musicbrainz.LookupFields). Restano fuori i nomi da cui non si
// ricaverebbero titolo e artista e quello che il popup propone comunque.
// Niente ricerca (e nessun nome) se è disattivata (Config.MusicBrainz) o per
// un file non mp3. Gli errori non finiscono nel registro: li mostra il popup.
func (a *App) SuggestTrackNames(path string) TrackSuggestions {
	a.mu.Lock()
	cfg := a.config
	a.mu.Unlock()
	none := TrackSuggestions{Names: []string{}}
	if !cfg.MusicBrainz {
		return none
	}
	title, artist, unknown := unknownTagFor(path, cfg)
	if title == "" && artist == "" {
		return none // non mp3: niente tag, niente popup
	}

	original := parser.RemoveExtension(filepath.Base(path))
	normalized := cfg.NormalizeFileBase(original)
	// key identifica la ricerca nella cache; current è il nome che il popup
	// propone comunque (inutile riproporlo).
	key, current := "titolo\x00"+title+"\x00"+artist, normalized
	if unknown {
		key, current = "nome\x00"+normalized, original
	}

	a.mu.Lock()
	found, ok := a.suggestions[key]
	a.mu.Unlock()
	if !ok {
		ctx, cancel := context.WithTimeout(context.Background(), suggestTimeout)
		defer cancel()
		var recs []musicbrainz.Recording
		var err error
		if unknown {
			recs, err = musicbrainz.Lookup(ctx, normalized)
		} else {
			recs, err = musicbrainz.LookupFields(ctx, title, artist)
		}
		if err != nil {
			// Errori (offline) non in cache: si riprova al prossimo popup.
			return TrackSuggestions{Names: []string{}, Error: err.Error()}
		}
		for _, r := range recs {
			found = append(found, r.Base(cfg.FtDestination()))
		}
		a.mu.Lock()
		if a.suggestions == nil {
			a.suggestions = make(map[string][]string)
		}
		a.suggestions[key] = found
		a.mu.Unlock()
	}

	ext := parser.Extension(filepath.Base(path))
	seen := map[string]bool{current: true}
	out := none
	for _, f := range found {
		base := cfg.NormalizeFileBase(f)
		name := base + "." + ext
		if seen[base] ||
			parser.TagTitle(name, cfg.FtDestination(), cfg.ArtistExceptions) == parser.UnknownTitle ||
			parser.TagArtist(name, cfg.FtDestination(), cfg.ArtistExceptions) == parser.UnknownArtist {
			continue
		}
		seen[base] = true
		out.Names = append(out.Names, base)
	}
	return out
}
