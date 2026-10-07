package core

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"renamemusic/internal/musicbrainz"
	"renamemusic/internal/rules"
)

// fakeMusicBrainz sostituisce l'API di MusicBrainz con una risposta fissa (o
// un errore HTTP se status non è 200) e conta le richieste ricevute.
func fakeMusicBrainz(t *testing.T, status int, body string) *int {
	t.Helper()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	oldURL, oldInterval := musicbrainz.BaseURL, musicbrainz.MinInterval
	musicbrainz.BaseURL, musicbrainz.MinInterval = srv.URL, 0
	t.Cleanup(func() {
		srv.Close()
		musicbrainz.BaseURL, musicbrainz.MinInterval = oldURL, oldInterval
	})
	return &calls
}

const levelsResponse = `{"recordings":[
	{"title":"Levels","score":100,"artist-credit":[{"name":"Avicii","joinphrase":""}],"releases":[{},{}]},
	{"title":"Levels","score":100,"artist-credit":[{"name":"Nick Gunner","joinphrase":""}],"releases":[{}]}]}`

func TestSuggestTrackNamesUnknown(t *testing.T) {
	calls := fakeMusicBrainz(t, http.StatusOK, levelsResponse)
	app := &App{config: rules.FactoryConfig()}
	path := `C:\musica\Avicii Levels (Official Video).mp3`

	got := app.SuggestTrackNames(path)
	if got.Error != "" || strings.Join(got.Names, "|") != "Avicii - Levels" {
		t.Fatalf("SuggestTrackNames = %+v", got)
	}
	// Seconda volta (ricerca in anticipo + popup): dalla cache.
	if again := app.SuggestTrackNames(path); strings.Join(again.Names, "|") != "Avicii - Levels" || *calls != 1 {
		t.Fatalf("SuggestTrackNames = %+v con %d richieste, attesa 1", again, *calls)
	}
}

func TestSuggestTrackNamesTitleOnly(t *testing.T) {
	fakeMusicBrainz(t, http.StatusOK, levelsResponse)
	app := &App{config: rules.FactoryConfig()}

	got := app.SuggestTrackNames(`C:\musica\Levels.mp3`)
	if strings.Join(got.Names, "|") != "Avicii - Levels|Nick Gunner - Levels" {
		t.Fatalf("SuggestTrackNames = %+v", got)
	}
}

func TestSuggestTrackNamesReview(t *testing.T) {
	fakeMusicBrainz(t, http.StatusOK, levelsResponse)
	app := &App{config: rules.FactoryConfig()}

	// Stesso nome dell'anteprima: nessuna proposta.
	if got := app.SuggestTrackNames(`C:\musica\Avicii - Levels.mp3`); len(got.Names) != 0 {
		t.Fatalf("SuggestTrackNames = %+v, atteso vuoto", got)
	}
	// Grafia diversa: si propone quella di MusicBrainz.
	if got := app.SuggestTrackNames(`C:\musica\AVICII - levels.mp3`); strings.Join(got.Names, "|") != "Avicii - Levels" {
		t.Fatalf("SuggestTrackNames = %+v", got)
	}
}

func TestSuggestTrackNamesError(t *testing.T) {
	calls := fakeMusicBrainz(t, http.StatusInternalServerError, "")
	app := &App{config: rules.FactoryConfig()}
	path := `C:\musica\Avicii Levels.mp3`

	if got := app.SuggestTrackNames(path); got.Error == "" || len(got.Names) != 0 {
		t.Fatalf("SuggestTrackNames = %+v, atteso un errore", got)
	}
	// Gli errori non finiscono in cache: la volta dopo si riprova.
	app.SuggestTrackNames(path)
	if *calls != 2 {
		t.Fatalf("richieste = %d, attese 2", *calls)
	}
}

func TestSuggestTrackNamesDisabled(t *testing.T) {
	calls := fakeMusicBrainz(t, http.StatusOK, levelsResponse)
	cfg := rules.FactoryConfig()
	cfg.MusicBrainz = false
	app := &App{config: cfg}

	if got := app.SuggestTrackNames(`C:\musica\Avicii Levels.mp3`); len(got.Names) != 0 || *calls != 0 {
		t.Fatalf("SuggestTrackNames = %+v con %d richieste, attesi vuoto e 0", got, *calls)
	}
	// Non mp3: nessuna richiesta.
	app.config.MusicBrainz = true
	if got := app.SuggestTrackNames(`C:\musica\Avicii Levels.flac`); len(got.Names) != 0 || *calls != 0 {
		t.Fatalf("SuggestTrackNames = %+v con %d richieste, attesi vuoto e 0", got, *calls)
	}
}
