package musicbrainz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func rec(title string, score int, credits ...Credit) Recording {
	return Recording{Title: title, Score: score, Artists: credits}
}

// withReleases imposta il numero di uscite di r.
func withReleases(r Recording, n int) Recording {
	r.Releases = make([]struct{}, n)
	return r
}

func bases(recs []Recording) []string {
	out := make([]string, len(recs))
	for i, r := range recs {
		out[i] = r.Base("ft")
	}
	return out
}

// fakeAPI risponde con il corpo della prima query di responses che è prefisso
// della query ricevuta (nessun risultato altrimenti) e registra le query.
func fakeAPI(t *testing.T, responses [][2]string) *[]string {
	t.Helper()
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		queries = append(queries, q.Get("query"))
		if r.URL.Path != "/recording" || q.Get("fmt") != "json" || !strings.HasPrefix(r.UserAgent(), "RenameMusic/") {
			t.Errorf("richiesta inattesa: %s %s, User-Agent %q", r.URL.Path, r.URL.RawQuery, r.UserAgent())
		}
		for _, resp := range responses {
			if strings.HasPrefix(q.Get("query"), resp[0]) {
				w.Write([]byte(resp[1]))
				return
			}
		}
		w.Write([]byte(`{"recordings":[]}`))
	}))
	oldURL, oldInterval := BaseURL, MinInterval
	BaseURL, MinInterval = srv.URL, 0
	t.Cleanup(func() {
		srv.Close()
		BaseURL, MinInterval = oldURL, oldInterval
	})
	return &queries
}

// TestLookupFallsBackToArtist: la prima ricerca restituisce solo remix (da
// scartare), nessun brano si intitola come il nome intero, ma c'è l'artista con
// cui inizia il nome: l'ultima ricerca cerca il resto come titolo suo.
func TestLookupFallsBackToArtist(t *testing.T) {
	queries := fakeAPI(t, [][2]string{
		{`recording:"get lucky" AND artist:"daft punk"`, `{"recordings":[{"title":"Get Lucky","score":100,"artist-credit":[
			{"name":"Daft Punk","joinphrase":" feat. "},{"name":"Pharrell Williams","joinphrase":""}]}]}`},
		{`recording:(`, `{"recordings":[
			{"title":"Get Lucky (Daft Punk remix)","score":100,"artist-credit":[{"name":"Daft Punk","joinphrase":""}]},
			{"title":"Daft Punk Megamix","score":84,"artist-credit":[{"name":"Daft Punk","joinphrase":""}]}]}`},
	})

	recs, err := Lookup(context.Background(), "Daft Punk Get Lucky")
	if err != nil {
		t.Fatal(err)
	}
	if got := bases(recs); len(got) != 1 || got[0] != "Daft Punk - Get Lucky ft Pharrell Williams" {
		t.Fatalf("Lookup = %q", got)
	}
	want := []string{
		"recording:(daft punk get lucky) AND artist:(daft punk get lucky)",
		`recording:"daft punk get lucky"`,
		`recording:"get lucky" AND artist:"daft punk"`,
	}
	if strings.Join(*queries, "\n") != strings.Join(want, "\n") {
		t.Fatalf("query = %q, attese %q", *queries, want)
	}
}

// TestLookupTitleOnly: un nome con il solo titolo dà i brani con quel titolo,
// riuniti per artista e dal più diffuso.
func TestLookupTitleOnly(t *testing.T) {
	fakeAPI(t, [][2]string{
		{`recording:"say it back"`, `{"recordings":[
			{"title":"Say It Back","score":100,"artist-credit":[{"name":"Matilda Mann"}],"releases":[{},{},{},{}]},
			{"title":"I Love You Say It Back","score":100,"artist-credit":[{"name":"Mungbean"}],"releases":[{},{},{},{},{},{}]},
			{"title":"Say It Back","score":100,"artist-credit":[{"name":"d4vd"}],"releases":[{},{},{}]},
			{"title":"say it back","score":100,"artist-credit":[{"name":"d4vd"}],"releases":[{},{}]}]}`},
	})

	recs, err := Lookup(context.Background(), "Say It Back")
	if err != nil {
		t.Fatal(err)
	}
	got := bases(recs)
	if strings.Join(got, "|") != "d4vd - Say It Back|Matilda Mann - Say It Back" {
		t.Fatalf("Lookup = %q", got)
	}
}

func TestSearchRetriesOnceOn503(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	oldURL, oldInterval := BaseURL, MinInterval
	BaseURL, MinInterval = srv.URL, 0
	t.Cleanup(func() { BaseURL, MinInterval = oldURL, oldInterval })

	if _, err := Search(context.Background(), "x", 1); err == nil {
		t.Fatal("atteso un errore")
	}
	if calls != 2 {
		t.Fatalf("richieste = %d, attese 2", calls)
	}
}

func TestRemainder(t *testing.T) {
	words := tokens("Daft Punk Get Lucky")
	if rest, ok := remainder(words, tokens("Daft Punk")); !ok || strings.Join(rest, " ") != "get lucky" {
		t.Fatalf("inizio: %q, %v", rest, ok)
	}
	if rest, ok := remainder(words, tokens("Get Lucky")); !ok || strings.Join(rest, " ") != "daft punk" {
		t.Fatalf("fine: %q, %v", rest, ok)
	}
	for _, artist := range []string{"Punk", "Daft Punk Get Lucky", ""} {
		if rest, ok := remainder(words, tokens(artist)); ok {
			t.Fatalf("%q: atteso nessun resto, ottenuto %q", artist, rest)
		}
	}
}

func TestMatches(t *testing.T) {
	recs := []Recording{
		rec("Levels (Skrillex remix)", 100, Credit{Name: "Avicii"}), // parole del titolo assenti nel nome
		withReleases(rec("Levels", 95, Credit{Name: "Avicii"}), 1),
		rec("Levels", 100, Credit{Name: "Nick Gunner"}),            // artista assente nel nome
		withReleases(rec("Levels", 90, Credit{Name: "Avicii"}), 3), // stesso brano: si riunisce
		rec("Avicii", 100, Credit{Name: "Levels"}),                 // stessa copertura, meno diffuso
	}
	got := bases(Matches(recs, "Avicii Levels"))
	if strings.Join(got, "|") != "Avicii - Levels|Levels - Avicii" {
		t.Fatalf("Matches = %q", got)
	}
	// Copertura insufficiente: il risultato spiega solo una parola su quattro.
	if got := Matches([]Recording{rec("Levels", 100, Credit{Name: "Levels"})}, "Avicii Levels live Ushuaia"); len(got) != 0 {
		t.Fatalf("atteso nessun risultato, ottenuto %q", bases(got))
	}
	// Diacritici e maiuscole non contano.
	if got := Matches([]Recording{rec("Halo", 100, Credit{Name: "Beyoncé"})}, "beyonce HALO"); len(got) != 1 {
		t.Fatal("atteso un risultato con i diacritici ignorati")
	}
}

func TestSimilar(t *testing.T) {
	recs := []Recording{
		rec("Something Else", 100, Credit{Name: "Other"}),
		rec("Don't Stop Me Now", 95, Credit{Name: "Queen"}),
		rec("Don't Stop Me Now", 80, Credit{Name: "Cover Band Queen"}), // punteggio sotto 90
	}
	got := bases(Similar(recs, "Dont Stop Me Now Queen"))
	if len(got) != 1 || got[0] != "Queen - Don't Stop Me Now" {
		t.Fatalf("Similar = %q", got)
	}
}

// TestRankKeepsAll: nessun limite al numero di brani, uno per artista.
func TestRankKeepsAll(t *testing.T) {
	var recs []Recording
	for i := 0; i < 40; i++ {
		recs = append(recs, rec("Song", 100, Credit{Name: "Artista " + string(rune('A'+i%20))}))
	}
	if got := SameTitle(recs, "song"); len(got) != 20 {
		t.Fatalf("SameTitle = %d brani, attesi 20", len(got))
	}
}

func TestBase(t *testing.T) {
	cases := []struct {
		rec  Recording
		want string
	}{
		{rec("Levels", 0, Credit{Name: "Avicii"}), "Avicii - Levels"},
		{rec("Lonely Together", 0, Credit{Name: "Avicii", JoinPhrase: " feat. "}, Credit{Name: "Rita Ora"}), "Avicii - Lonely Together ft Rita Ora"},
		{rec("Title", 0, Credit{Name: "A", JoinPhrase: " & "}, Credit{Name: "B", JoinPhrase: " feat. "}, Credit{Name: "C", JoinPhrase: " & "}, Credit{Name: "D"}), "A & B - Title ft C & D"},
		{rec("What? / Why: Mix - Radio Edit", 0, Credit{Name: "AC/DC"}), "AC DC - What Why Mix Radio Edit"},
	}
	for _, c := range cases {
		if got := c.rec.Base("ft"); got != c.want {
			t.Errorf("Base() = %q, atteso %q", got, c.want)
		}
	}
}
