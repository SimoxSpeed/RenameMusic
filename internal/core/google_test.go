package core

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"renamemusic/internal/drive"
	"renamemusic/internal/google"
	"renamemusic/internal/playlist"
	"renamemusic/internal/rules"
	"renamemusic/internal/youtube"
)

// fakeGoogleAuth è un accesso a Google già concesso (o, con signedOut, non
// più valido).
type fakeGoogleAuth struct {
	signedOut bool
}

func (fakeGoogleAuth) Configured() bool              { return true }
func (fakeGoogleAuth) SignIn(context.Context) error  { return nil }
func (fakeGoogleAuth) Invalidate(string)             {}
func (fakeGoogleAuth) SignOut(context.Context) error { return nil }
func (f fakeGoogleAuth) Token(context.Context) (string, error) {
	if f.signedOut {
		return "", google.ErrSignedOut
	}
	return "tok", nil
}

// fakeYouTube simula la YouTube Data API per la playlist "PLcoda" (v1 compare
// due volte, v2 non è stato scaricato) e per "PLarchivio" (contiene già v9).
type fakeYouTube struct {
	mu       sync.Mutex
	deleted  []string
	inserted []string
	// driveFile è il contenuto di playlist-prefs.json nella cartella dati
	// dell'app su Drive (nil = non esiste).
	driveFile []byte
	writes    int
}

// serveDrive simula la Drive API per la cartella dati dell'app (un solo file).
func (f *fakeYouTube) serveDrive(w http.ResponseWriter, r *http.Request) bool {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/files":
		if f.driveFile == nil {
			io.WriteString(w, `{"files":[]}`)
		} else {
			io.WriteString(w, `{"files":[{"id":"prefs"}]}`)
		}
	case r.Method == http.MethodGet && r.URL.Path == "/files/prefs":
		w.Write(f.driveFile)
	case r.Method == http.MethodPatch && r.URL.Path == "/upload/files/prefs":
		f.driveFile, _ = io.ReadAll(r.Body)
		f.writes++
		io.WriteString(w, `{}`)
	case r.Method == http.MethodPost && r.URL.Path == "/upload/files":
		_, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		mr := multipart.NewReader(r.Body, params["boundary"])
		var parts [][]byte
		for {
			p, err := mr.NextPart()
			if err != nil {
				break
			}
			data, _ := io.ReadAll(p)
			parts = append(parts, data)
		}
		if len(parts) == 2 && strings.Contains(string(parts[0]), "appDataFolder") {
			f.driveFile = parts[1]
			f.writes++
		}
		io.WriteString(w, `{"id":"prefs"}`)
	default:
		return false
	}
	return true
}

// driveWrites restituisce quante volte è stato scritto il file su Drive e il
// suo contenuto attuale.
func (f *fakeYouTube) driveState() (int, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.writes, string(f.driveFile)
}

func (f *fakeYouTube) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.serveDrive(w, r) {
		return
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/playlistItems" && r.URL.Query().Get("playlistId") == "PLcoda":
		io.WriteString(w, `{"items":[
			{"id":"i1","snippet":{"resourceId":{"videoId":"v1"}}},
			{"id":"i2","snippet":{"resourceId":{"videoId":"v2"}}},
			{"id":"i3","snippet":{"resourceId":{"videoId":"v1"}}}]}`)
	case r.Method == http.MethodGet && r.URL.Path == "/playlistItems" && r.URL.Query().Get("playlistId") == "PLarchivio":
		io.WriteString(w, `{"items":[{"id":"a1","snippet":{"resourceId":{"videoId":"v9"}}}]}`)
	case r.Method == http.MethodGet && r.URL.Path == "/playlists":
		io.WriteString(w, `{"items":[{"id":"PLcoda","snippet":{"title":"Coda"}},{"id":"PLarchivio","snippet":{"title":"Archivio"}}]}`)
	case r.Method == http.MethodPost && r.URL.Path == "/playlistItems":
		body, _ := io.ReadAll(r.Body)
		f.inserted = append(f.inserted, string(body))
		io.WriteString(w, `{}`)
	case r.Method == http.MethodDelete && r.URL.Path == "/playlistItems":
		f.deleted = append(f.deleted, r.URL.Query().Get("id"))
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func newQueueApp(t *testing.T, auth GoogleAuth, connected bool) (*App, *fakeYouTube, string) {
	t.Helper()
	api := &fakeYouTube{}
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	old := youtube.BaseURL
	youtube.BaseURL = srv.URL
	t.Cleanup(func() { youtube.BaseURL = old })
	oldDrive, oldUpload := drive.BaseURL, drive.UploadURL
	drive.BaseURL, drive.UploadURL = srv.URL, srv.URL+"/upload"
	t.Cleanup(func() { drive.BaseURL, drive.UploadURL = oldDrive, oldUpload })

	runner := &fakeRunner{titles: map[string]string{"v1": "Artista - Titolo"}}
	app, musicDir := newSimpleApp(t, runner, nil)
	app.google = auth
	app.googleConnected = connected
	// Prima di chiudere il server finto si attendono le sincronizzazioni in
	// background (le Cleanup girano in ordine inverso).
	t.Cleanup(app.syncWG.Wait)
	app.playlists = []playlist.Playlist{{Name: "Coda", URL: "https://www.youtube.com/playlist?list=PLcoda"}}
	app.playlistPrefs = map[string]playlist.Prefs{"yt:PLcoda": {AfterDownload: playlist.AfterRemove}}
	return app, api, musicDir
}

// TestRemoveAfterDownload verifica che, dopo "Scarica e converti" di una
// playlist usata come coda, si tolgano da YouTube tutte le copie dei soli
// video scaricati.
func TestRemoveAfterDownload(t *testing.T) {
	app, api, musicDir := newQueueApp(t, fakeGoogleAuth{}, true)

	resp := app.DownloadAndProcess("Coda")
	if !resp.OK {
		t.Fatalf("DownloadAndProcess non ok: %s", resp.Message)
	}
	slices.Sort(api.deleted)
	if !slices.Equal(api.deleted, []string{"i1", "i3"}) {
		t.Fatalf("elementi rimossi %v, attesi [i1 i3]", api.deleted)
	}
	if !strings.Contains(resp.Message, "1 brano tolto") {
		t.Errorf("messaggio senza la nota sulla playlist: %q", resp.Message)
	}
	if _, err := os.Stat(filepath.Join(musicDir, "Artista - Titolo.mp3")); err != nil {
		t.Fatalf("file convertito assente: %v", err)
	}
}

// Senza account collegato i brani si scaricano e convertono comunque, ma
// l'esito segnala che la playlist non è stata svuotata.
func TestRemoveAfterDownloadNotConnected(t *testing.T) {
	app, api, musicDir := newQueueApp(t, fakeGoogleAuth{}, false)

	resp := app.DownloadAndProcess("Coda")
	if resp.OK || !strings.Contains(resp.Message, "non svuotata") {
		t.Fatalf("atteso avviso di playlist non svuotata, ottenuto ok=%v %q", resp.OK, resp.Message)
	}
	if len(api.deleted) != 0 {
		t.Fatalf("nessuna rimozione attesa, eseguite %v", api.deleted)
	}
	if _, err := os.Stat(filepath.Join(musicDir, "Artista - Titolo.mp3")); err != nil {
		t.Fatalf("la conversione doveva avvenire comunque: %v", err)
	}
}

// Un accesso non più valido scollega l'account.
func TestRemoveAfterDownloadSignedOut(t *testing.T) {
	app, _, _ := newQueueApp(t, fakeGoogleAuth{signedOut: true}, true)

	resp := app.DownloadAndProcess("Coda")
	if resp.OK {
		t.Fatal("atteso errore con l'accesso scaduto")
	}
	if resp.State.Google.Connected {
		t.Fatal("l'account doveva risultare scollegato")
	}
}

// Una playlist dell'account si scarica dalla scelta del download; lo
// «Svuota dopo il download» viene da una playlist salvata con lo stesso ID.
func TestDownloadGooglePlaylist(t *testing.T) {
	app, api, musicDir := newQueueApp(t, fakeGoogleAuth{}, true)
	app.googleLists = []GooglePlaylistView{{ID: "PLcoda", Title: "Coda su YouTube", URL: youtube.PlaylistURL("PLcoda")}}

	if resp := app.DownloadGooglePlaylist("PLaltro"); resp.OK {
		t.Fatal("una playlist non dell'account doveva essere rifiutata")
	}
	resp := app.DownloadGooglePlaylistAndProcess("PLcoda")
	if !resp.OK {
		t.Fatalf("DownloadGooglePlaylistAndProcess non ok: %s", resp.Message)
	}
	if len(api.deleted) != 2 {
		t.Fatalf("attese 2 rimozioni (flag della playlist salvata), eseguite %v", api.deleted)
	}
	if _, err := os.Stat(filepath.Join(musicDir, "Artista - Titolo.mp3")); err != nil {
		t.Fatalf("file convertito assente: %v", err)
	}
	if got := resp.State.Google.Playlists; len(got) != 1 || got[0].ID != "PLcoda" {
		t.Fatalf("playlist dell'account nello stato inattese: %+v", got)
	}
}

// «Aggiungi a un'altra playlist»: i brani scaricati finiscono in fondo alla
// playlist di destinazione, senza toccare quella di partenza né duplicare.
func TestCopyAfterDownload(t *testing.T) {
	app, api, _ := newQueueApp(t, fakeGoogleAuth{}, true)
	app.playlistPrefs = map[string]playlist.Prefs{
		"yt:PLcoda": {AfterDownload: playlist.AfterCopy, CopyTo: "PLarchivio", CopyToTitle: "Archivio"},
	}

	resp := app.DownloadAndProcess("Coda")
	if !resp.OK {
		t.Fatalf("DownloadAndProcess non ok: %s", resp.Message)
	}
	if len(api.deleted) != 0 {
		t.Fatalf("nessuna rimozione attesa, eseguite %v", api.deleted)
	}
	if len(api.inserted) != 1 || !strings.Contains(api.inserted[0], `"playlistId":"PLarchivio"`) || !strings.Contains(api.inserted[0], `"videoId":"v1"`) {
		t.Fatalf("aggiunta inattesa: %v", api.inserted)
	}
	if !strings.Contains(resp.Message, `aggiunto alla playlist "Archivio"`) {
		t.Errorf("messaggio senza la nota sull'aggiunta: %q", resp.Message)
	}
}

// «Svuota ora» toglie tutti i brani di una playlist dell'account, e solo di
// quelle.
func TestEmptyGooglePlaylist(t *testing.T) {
	app, api, _ := newQueueApp(t, fakeGoogleAuth{}, true)
	app.googleLists = []GooglePlaylistView{{ID: "PLcoda", Title: "Coda"}}

	if resp := app.EmptyGooglePlaylist("PLaltro"); resp.OK {
		t.Fatal("una playlist non dell'account doveva essere rifiutata")
	}
	resp := app.EmptyGooglePlaylist("PLcoda")
	if !resp.OK {
		t.Fatalf("EmptyGooglePlaylist non ok: %s", resp.Message)
	}
	slices.Sort(api.deleted)
	if !slices.Equal(api.deleted, []string{"i1", "i2", "i3"}) {
		t.Fatalf("elementi rimossi %v, attesi tutti", api.deleted)
	}
}

func TestSetPlaylistPrefs(t *testing.T) {
	app, _, _ := newQueueApp(t, fakeGoogleAuth{}, true)

	bad := map[string]playlist.Prefs{
		"yt:PLcoda":  {AfterDownload: playlist.AfterCopy},                   // destinazione mancante
		"yt:PLarch":  {AfterDownload: playlist.AfterCopy, CopyTo: "PLarch"}, // sé stessa
		"pl:Manuale": {AfterDownload: playlist.AfterRemove},                 // non di YouTube
		"yt:PLcoda ": {AfterDownload: "boh"},
		"senza-tipo": {Hidden: true},
	}
	for key, p := range bad {
		if resp := app.SetPlaylistPrefs(key, p); resp.OK {
			t.Errorf("impostazioni %q %+v accettate", key, p)
		}
	}

	resp := app.SetPlaylistPrefs("yt:PLcoda", playlist.Prefs{Hidden: true, AfterDownload: playlist.AfterNothing, CopyTo: "x"})
	if !resp.OK {
		t.Fatalf("SetPlaylistPrefs non ok: %s", resp.Message)
	}
	if got := resp.State.PlaylistPrefs["yt:PLcoda"]; got != (playlist.Prefs{Hidden: true}) {
		t.Fatalf("impostazioni salvate inattese: %+v", got)
	}
	if resp := app.SetPlaylistPrefs("yt:PLcoda", playlist.Prefs{}); !resp.OK || len(resp.State.PlaylistPrefs) != 0 {
		t.Fatalf("le impostazioni predefinite dovevano togliere la voce: %+v", resp.State.PlaylistPrefs)
	}
}

// Con «Togli anche dalla playlist di origine» i brani si spostano: aggiunti
// alla destinazione e poi tolti (tutte le copie) dall'origine.
func TestMoveAfterDownload(t *testing.T) {
	app, api, _ := newQueueApp(t, fakeGoogleAuth{}, true)
	app.playlistPrefs = map[string]playlist.Prefs{
		"yt:PLcoda": {AfterDownload: playlist.AfterCopy, CopyTo: "PLarchivio", CopyToTitle: "Archivio", MoveOnCopy: true},
	}

	resp := app.DownloadAndProcess("Coda")
	if !resp.OK {
		t.Fatalf("DownloadAndProcess non ok: %s", resp.Message)
	}
	if len(api.inserted) != 1 {
		t.Fatalf("attesa 1 aggiunta, eseguite %v", api.inserted)
	}
	slices.Sort(api.deleted)
	if !slices.Equal(api.deleted, []string{"i1", "i3"}) {
		t.Fatalf("elementi tolti dall'origine %v, attesi [i1 i3]", api.deleted)
	}
	if !strings.Contains(resp.Message, "aggiunto") || !strings.Contains(resp.Message, "tolto") {
		t.Errorf("messaggio incompleto: %q", resp.Message)
	}

	// Una playlist salvata senza ID non si può svuotare.
	if r := app.SetPlaylistPrefs("pl:Manuale", playlist.Prefs{AfterDownload: playlist.AfterCopy, CopyTo: "PLarchivio", MoveOnCopy: true}); r.OK {
		t.Error("spostamento da una playlist non di YouTube accettato")
	}
}

// driveSections legge le sezioni del file delle impostazioni sull'account.
func driveSections(t *testing.T, api *fakeYouTube) map[string]syncSection {
	t.Helper()
	_, content := api.driveState()
	var doc syncedSettings
	if err := json.Unmarshal([]byte(content), &doc); err != nil {
		t.Fatalf("file delle impostazioni non valido: %v\n%s", err, content)
	}
	return doc.Sections
}

// Le impostazioni cambiate su un dispositivo si caricano sull'account, tutte
// le sezioni, senza la cartella di partenza (che è del dispositivo).
func TestSettingsPushed(t *testing.T) {
	app, api, musicDir := newQueueApp(t, fakeGoogleAuth{}, true)

	if resp := app.SetPlaylistPrefs("yt:PLcoda", playlist.Prefs{Hidden: true}); !resp.OK {
		t.Fatalf("SetPlaylistPrefs non ok: %s", resp.Message)
	}
	app.syncWG.Wait()
	sections := driveSections(t, api)
	for _, s := range syncSections {
		if _, ok := sections[s]; !ok {
			t.Errorf("sezione %q non caricata", s)
		}
	}
	var prefs map[string]playlist.Prefs
	_ = json.Unmarshal(sections[syncPrefs].Data, &prefs)
	if !prefs["yt:PLcoda"].Hidden {
		t.Fatalf("impostazioni delle playlist non caricate: %s", sections[syncPrefs].Data)
	}
	var cfg rules.Config
	_ = json.Unmarshal(sections[syncConfig].Data, &cfg)
	if cfg.StartFolder != "" || musicDir == "" {
		t.Fatalf("la cartella di partenza non deve finire sull'account: %q", cfg.StartFolder)
	}
	if len(app.syncDirty) != 0 || app.syncErr != "" {
		t.Fatalf("dopo il caricamento: dirty=%v, errore %q", app.syncDirty, app.syncErr)
	}
}

// Le sezioni più recenti sull'account (cambiate da un altro dispositivo)
// sostituiscono le locali, sezione per sezione; quelle locali più recenti si
// caricano. Le regole ricevute tengono la cartella di partenza del dispositivo.
func TestSettingsFromAccount(t *testing.T) {
	app, api, musicDir := newQueueApp(t, fakeGoogleAuth{}, true)
	old := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	app.syncUpdated = map[string]time.Time{syncPrefs: old, syncConfig: old, syncPlaylists: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
	api.driveFile = []byte(`{"version":1,"sections":{
		"playlistPrefs":{"updated":"2026-09-30T10:00:00Z","data":{"yt:PLarchivio":{"hidden":true}}},
		"config":{"updated":"2026-09-30T10:00:00Z","data":{"simpleMode":false,"ftAlias":"feat","startFolder":"C:/altro"}},
		"playlists":{"updated":"2026-09-30T10:00:00Z","data":[{"name":"Altrove","url":"https://www.youtube.com/playlist?list=PLx"}]}}}`)

	if !app.syncSettings(context.Background()) {
		t.Fatal("le impostazioni dell'account dovevano sostituire le locali")
	}
	if !app.playlistPrefs["yt:PLarchivio"].Hidden || len(app.playlistPrefs) != 1 {
		t.Fatalf("impostazioni delle playlist inattese: %+v", app.playlistPrefs)
	}
	if app.config.FtAlias != "feat" || app.config.SimpleMode || app.config.StartFolder != musicDir {
		t.Fatalf("regole ricevute inattese: alias %q, semplificata %v, cartella %q", app.config.FtAlias, app.config.SimpleMode, app.config.StartFolder)
	}
	// Le playlist locali erano più recenti: restano, e vanno sull'account.
	if len(app.playlists) != 1 || app.playlists[0].Name != "Coda" {
		t.Fatalf("le playlist locali più recenti dovevano restare: %+v", app.playlists)
	}
	var remote []playlist.Playlist
	_ = json.Unmarshal(driveSections(t, api)[syncPlaylists].Data, &remote)
	if len(remote) != 1 || remote[0].Name != "Coda" {
		t.Fatalf("sull'account dovevano finire le playlist locali: %+v", remote)
	}
}

// Senza account collegato non si sincronizza nulla.
func TestSettingsNotSyncedWhenSignedOut(t *testing.T) {
	app, api, _ := newQueueApp(t, fakeGoogleAuth{}, false)
	app.SetPlaylistPrefs("yt:PLcoda", playlist.Prefs{Hidden: true})
	app.syncWG.Wait()
	if writes, _ := api.driveState(); writes != 0 {
		t.Fatalf("senza account non doveva caricare nulla (%d scritture)", writes)
	}
	if !app.syncDirty[syncPrefs] {
		t.Fatal("la modifica doveva restare da caricare al prossimo collegamento")
	}
}
