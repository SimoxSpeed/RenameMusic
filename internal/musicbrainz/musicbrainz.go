// Package musicbrainz è un client minimo della ricerca di MusicBrainz
// (https://musicbrainz.org/doc/MusicBrainz_API/Search), limitato a ciò che
// serve all'app: cercare una registrazione (titolo + artisti) a partire dal
// nome di una traccia, per proporre "Artista - Titolo" quando il nome non
// basta. Chiamate REST dirette, come youtube: gira anche con gomobile.
//
// Regole del servizio: al massimo una richiesta al secondo per IP e uno
// User-Agent che identifichi l'app (vedi wait e UserAgent).
package musicbrainz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"renamemusic/internal/update"
)

// BaseURL è l'indirizzo dell'API (variabile per i test, anche di altri package).
var BaseURL = "https://musicbrainz.org/ws/2"

// UserAgent identifica l'app, come chiede MusicBrainz.
var UserAgent = "RenameMusic/" + update.Version + " ( https://github.com/SimoxSpeed/RenameMusic )"

// MinInterval è l'intervallo minimo fra due richieste (MusicBrainz ne accetta
// una al secondo; un po' di margine evita i 503). Variabile per i test.
var MinInterval = 1100 * time.Millisecond

var httpClient = &http.Client{Timeout: 15 * time.Second}

// Credit è un artista accreditato di una registrazione; JoinPhrase è il testo
// che lo separa dal successivo (" & ", " feat. ", ...).
type Credit struct {
	Name       string `json:"name"`
	JoinPhrase string `json:"joinphrase"`
}

// Recording è un risultato della ricerca; Score (0-100) è la pertinenza
// secondo MusicBrainz, Releases le uscite (album, singoli, raccolte) che la
// contengono: servono solo a contarle, per capire quale brano è più diffuso.
type Recording struct {
	Title    string     `json:"title"`
	Artists  []Credit   `json:"artist-credit"`
	Score    int        `json:"score"`
	Releases []struct{} `json:"releases"`
}

// pageSize è il numero di registrazioni chieste a ogni ricerca: il massimo
// che MusicBrainz restituisce in una richiesta.
const pageSize = 100

var (
	limitMu  sync.Mutex
	lastCall time.Time
)

// wait rispetta il limite di una richiesta al secondo, condiviso da tutte le
// ricerche del processo (le richieste si mettono in fila).
func wait(ctx context.Context) error {
	limitMu.Lock()
	defer limitMu.Unlock()
	if d := MinInterval - time.Since(lastCall); d > 0 {
		t := time.NewTimer(d)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
	lastCall = time.Now()
	return nil
}

// Search cerca le registrazioni che corrispondono a query (sintassi Lucene),
// al massimo limit (fino a 100).
func Search(ctx context.Context, query string, limit int) ([]Recording, error) {
	q := url.Values{}
	q.Set("query", query)
	q.Set("fmt", "json")
	q.Set("limit", strconv.Itoa(limit))
	u := BaseURL + "/recording?" + q.Encode()

	for attempt := 0; ; attempt++ {
		if err := wait(ctx); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", UserAgent)
		req.Header.Set("Accept", "application/json")
		resp, err := httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, errors.New("MusicBrainz non ha risposto in tempo")
			}
			return nil, errors.New("MusicBrainz non raggiungibile: controlla la connessione")
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		// 503 = troppe richieste: si riprova una volta (wait rispetta l'intervallo).
		if resp.StatusCode == http.StatusServiceUnavailable && attempt == 0 {
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return nil, fmt.Errorf("MusicBrainz ha risposto HTTP %d", resp.StatusCode)
		}
		var body struct {
			Recordings []Recording `json:"recordings"`
		}
		if err := json.Unmarshal(data, &body); err != nil {
			return nil, fmt.Errorf("risposta di MusicBrainz non valida: %w", err)
		}
		return body.Recordings, nil
	}
}

// Lookup cerca i brani che corrispondono a un nome di cui non si sa quale
// parte sia l'artista e quale il titolo, dal più probabile (vuoto se non c'è
// nulla di attendibile). In ordine:
//
//  1. le stesse parole sia nel titolo sia nell'artista ("Avicii Levels"):
//     valgono i brani con titolo e artista contenuti nel nome (vedi Matches);
//  2. il nome come titolo ("Say It Back"): i brani con esattamente quel
//     titolo, di artisti diversi (vedi SameTitle);
//  3. un artista dei risultati del passo 1 con cui il nome inizia o finisce
//     ("Daft Punk Get Lucky", dove al passo 1 ci sono solo remix) e il resto
//     come titolo, al massimo due artisti.
//
// Ogni passo è una richiesta (una al secondo, fino a pageSize registrazioni):
// ci si ferma al primo che trova.
func Lookup(ctx context.Context, text string) ([]Recording, error) {
	words := tokens(text)
	if len(words) == 0 {
		return nil, nil
	}
	var recs []Recording
	if len(words) >= 2 {
		var err error
		if recs, err = Search(ctx, freeQuery(words), pageSize); err != nil {
			return nil, err
		}
		if found := Matches(recs, text); len(found) > 0 {
			return found, nil
		}
	}
	titled, err := Search(ctx, titleQuery(words), pageSize)
	if err != nil {
		return nil, err
	}
	if found := SameTitle(titled, text); len(found) > 0 {
		return found, nil
	}
	tried := map[string]bool{}
	for _, r := range recs {
		if len(tried) == 2 {
			break
		}
		if len(r.Artists) == 0 {
			continue
		}
		artist := tokens(r.Artists[0].Name)
		key := strings.Join(artist, " ")
		if tried[key] {
			continue
		}
		rest, ok := remainder(words, artist)
		if !ok {
			continue
		}
		tried[key] = true
		more, err := Search(ctx, fieldQuery(rest, artist), pageSize)
		if err != nil {
			return nil, err
		}
		if found := Matches(more, text); len(found) > 0 {
			return found, nil
		}
	}
	return nil, nil
}

// LookupFields cerca i brani di un titolo e un artista già dedotti dal nome,
// dal più probabile (vedi Similar).
func LookupFields(ctx context.Context, title, artist string) ([]Recording, error) {
	t, a := tokens(title), tokens(artist)
	if len(t) == 0 || len(a) == 0 {
		return nil, nil
	}
	recs, err := Search(ctx, fieldQuery(t, a), pageSize)
	if err != nil {
		return nil, err
	}
	return Similar(recs, title+" "+artist), nil
}

// titleQuery cerca le parole come titolo esatto (a meno di maiuscole e accenti).
func titleQuery(words []string) string {
	return `recording:"` + strings.Join(words, " ") + `"`
}

// freeQuery cerca le stesse parole in entrambi i campi (OR implicito fra le
// parole), così una parte cade nel titolo e una nell'artista. Le parole sono
// solo lettere e cifre: niente da proteggere della sintassi di Lucene.
func freeQuery(words []string) string {
	w := strings.Join(words, " ")
	return "recording:(" + w + ") AND artist:(" + w + ")"
}

// fieldQuery cerca un titolo e un artista noti.
func fieldQuery(title, artist []string) string {
	return `recording:"` + strings.Join(title, " ") + `" AND artist:"` + strings.Join(artist, " ") + `"`
}

// remainder restituisce le parole di words che restano togliendo artist
// dall'inizio o dalla fine; false se words non inizia né finisce con artist o
// se non resta nulla.
func remainder(words, artist []string) ([]string, bool) {
	n := len(artist)
	if n == 0 || n >= len(words) {
		return nil, false
	}
	if equalWords(words[:n], artist) {
		return words[n:], true
	}
	if equalWords(words[len(words)-n:], artist) {
		return words[:len(words)-n], true
	}
	return nil, false
}

func equalWords(a, b []string) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return len(a) == len(b)
}

// Matches tiene i brani di recs con titolo e primo artista contenuti per
// intero in text (il nome della traccia) e che ne spiegano almeno il 60% delle
// parole; dal più completo, a parità dal più diffuso (vedi rank).
func Matches(recs []Recording, text string) []Recording {
	textSet := set(tokens(text))
	return rank(recs, func(r Recording) (float64, bool) {
		titleTok := tokens(r.Title)
		if !containsAll(textSet, titleTok) || !containsAll(textSet, tokens(r.Artists[0].Name)) {
			return 0, false
		}
		coverage := float64(common(textSet, wordsOf(r))) / float64(len(textSet))
		return coverage, coverage >= 0.6
	})
}

// SameTitle tiene i brani di recs intitolati esattamente text (a meno di
// maiuscole, accenti e punteggiatura), di qualunque artista; dal più diffuso.
func SameTitle(recs []Recording, text string) []Recording {
	want := strings.Join(tokens(text), " ")
	return rank(recs, func(r Recording) (float64, bool) {
		return 0, strings.Join(tokens(r.Title), " ") == want
	})
}

// Similar tiene i brani di recs simili a text (titolo e artista già dedotti,
// da rivedere): punteggio di almeno 90 e parole in comune per almeno la metà
// (Jaccard), così passano anche correzioni di grafia e maiuscole; dal più
// simile, a parità dal più diffuso.
func Similar(recs []Recording, text string) []Recording {
	textSet := set(tokens(text))
	return rank(recs, func(r Recording) (float64, bool) {
		recSet := wordsOf(r)
		c := common(textSet, recSet)
		jaccard := float64(c) / float64(len(textSet)+len(recSet)-c)
		return jaccard, r.Score >= 90 && jaccard >= 0.5
	})
}

// rank filtra recs con keep (che dà anche la pertinenza), riunisce le
// registrazioni dello stesso brano (stessi artisti e titolo: singolo, album,
// versione rimasterizzata...) e le ordina per pertinenza, poi per diffusione:
// quante uscite contengono il brano, poi quante registrazioni ha.
func rank(recs []Recording, keep func(Recording) (float64, bool)) []Recording {
	type group struct {
		rec       Recording
		relevance float64
		releases  int
		count     int
	}
	var groups []*group
	byKey := map[string]*group{}
	for _, r := range recs {
		if len(tokens(r.Title)) == 0 || len(r.Artists) == 0 {
			continue
		}
		relevance, ok := keep(r)
		if !ok {
			continue
		}
		key := strings.Join(tokens(r.Base("ft")), " ")
		g := byKey[key]
		if g == nil {
			g = &group{rec: r, relevance: relevance}
			byKey[key] = g
			groups = append(groups, g)
		}
		g.releases += len(r.Releases)
		g.count++
	}
	sort.SliceStable(groups, func(i, j int) bool {
		a, b := groups[i], groups[j]
		if a.relevance != b.relevance {
			return a.relevance > b.relevance
		}
		if a.releases != b.releases {
			return a.releases > b.releases
		}
		return a.count > b.count
	})
	out := make([]Recording, len(groups))
	for i, g := range groups {
		out[i] = g.rec
	}
	return out
}

// wordsOf restituisce le parole di titolo e artisti di r.
func wordsOf(r Recording) map[string]bool {
	words := set(tokens(r.Title))
	for _, a := range r.Artists {
		for _, t := range tokens(a.Name) {
			words[t] = true
		}
	}
	return words
}

func common(a, b map[string]bool) int {
	n := 0
	for t := range b {
		if a[t] {
			n++
		}
	}
	return n
}

// Base compone il nome della traccia nel formato del parser:
// "Artisti - Titolo ft Ospiti". Gli ospiti sono gli artisti accreditati dopo
// il primo "feat."; ftAlias è il marcatore di featuring delle regole. I
// caratteri non validi nei nomi file diventano spazi.
func (r Recording) Base(ftAlias string) string {
	var main, guests strings.Builder
	inGuests := false
	for i, c := range r.Artists {
		if inGuests {
			guests.WriteString(c.Name)
		} else {
			main.WriteString(c.Name)
		}
		if i == len(r.Artists)-1 {
			break
		}
		if !inGuests && isFeat(c.JoinPhrase) {
			inGuests = true
			continue
		}
		if inGuests {
			guests.WriteString(c.JoinPhrase)
		} else {
			main.WriteString(c.JoinPhrase)
		}
	}
	// " - " nel titolo verrebbe preso per il separatore artista/titolo.
	title := strings.ReplaceAll(r.Title, " - ", " ")
	name := strings.TrimSpace(main.String()) + " - " + strings.TrimSpace(title)
	if g := strings.TrimSpace(guests.String()); g != "" {
		name += " " + ftAlias + " " + g
	}
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`\/:*?"<>|`, r) || unicode.IsControl(r) {
			return ' '
		}
		return r
	}, name)
	return strings.Join(strings.Fields(name), " ")
}

// isFeat indica una join phrase di featuring (" feat. ", " ft. ", " featuring ").
func isFeat(join string) bool {
	for _, w := range tokens(join) {
		if w == "feat" || w == "ft" || w == "featuring" {
			return true
		}
	}
	return false
}

// tokens divide s in parole minuscole senza diacritici (solo lettere e cifre).
func tokens(s string) []string {
	return strings.FieldsFunc(fold(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// fold porta s in minuscolo e toglie i diacritici delle lettere latine più
// comuni, così "Beyoncé" e "beyonce" coincidono.
func fold(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if f, ok := foldMap[r]; ok {
			b.WriteString(f)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var foldMap = func() map[rune]string {
	m := map[rune]string{'ß': "ss", 'æ': "ae", 'œ': "oe", 'ø': "o", 'ł': "l", 'đ': "d", 'ð': "d", 'þ': "th"}
	for base, variants := range map[string]string{
		"a": "àáâãäåāăą",
		"c": "çćĉċč",
		"e": "èéêëēĕėęě",
		"g": "ĝğġģ",
		"i": "ìíîïĩīĭįı",
		"n": "ñńņňŉ",
		"o": "òóôõöōŏő",
		"s": "śŝşšș",
		"t": "ţťț",
		"u": "ùúûüũūŭůűų",
		"y": "ýÿŷ",
		"z": "źżž",
	} {
		for _, v := range variants {
			m[v] = base
		}
	}
	return m
}()

func set(words []string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

func containsAll(s map[string]bool, words []string) bool {
	for _, w := range words {
		if !s[w] {
			return false
		}
	}
	return true
}
