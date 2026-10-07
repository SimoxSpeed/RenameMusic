package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"renamemusic/internal/google"
	"renamemusic/internal/playlist"
	"renamemusic/internal/youtube"
)

// EventGoogleChanged è emesso quando cambiano fuori da una richiesta della UI
// le playlist dell'account Google (lettura all'avvio) o il collegamento
// (accesso scaduto). Il payload è lo StateResponse aggiornato.
const EventGoogleChanged = "google:changed"

// GoogleView è lo stato dell'account Google. Available: l'accesso è
// disponibile su questa piattaforma/build; Email è l'indirizzo dell'account
// collegato ("" se collegato prima che l'app lo chiedesse). Playlists sono le
// playlist dell'account, proposte per prime nella scelta del download.
type GoogleView struct {
	Available bool                 `json:"available"`
	Connected bool                 `json:"connected"`
	Email     string               `json:"email"`
	Playlists []GooglePlaylistView `json:"playlists"`
	// SyncError: perché le impostazioni delle playlist non si sono potute
	// condividere con gli altri dispositivi dell'account ("" se a posto).
	SyncError string `json:"syncError,omitempty"`
	// SyncRevision cresce ogni volta che arrivano impostazioni cambiate su un
	// altro dispositivo: la UI lo avvisa (riparte da 0 all'avvio dell'app).
	SyncRevision int `json:"syncRevision"`
}

// GooglePlaylistView è una playlist dell'account Google collegato.
type GooglePlaylistView struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Count   int    `json:"count"`
	Privacy string `json:"privacy"`
}

const (
	// googleCallTimeout limita le chiamate brevi all'API (elenco playlist,
	// aggiunta di un video, scollegamento).
	googleCallTimeout = 45 * time.Second
	// googleRemoveTimeout limita la rimozione dei brani scaricati da una
	// playlist: una chiamata per brano.
	googleRemoveTimeout = 5 * time.Minute
	// googleListsMaxAge: al ritorno sulla finestra le playlist dell'account si
	// rileggono solo se l'ultima lettura è più vecchia di così (quota).
	googleListsMaxAge = 2 * time.Minute
)

// msgGoogleSignedOut è il messaggio per un accesso a Google non più valido.
const msgGoogleSignedOut = "Accesso a Google scaduto o revocato: ricollega l'account nelle Impostazioni."

// googleViewLocked costruisce la GoogleView. Va chiamata con il lock acquisito.
func (a *App) googleViewLocked() GoogleView {
	available := a.google != nil && a.google.Configured()
	v := GoogleView{
		Available: available,
		Connected: available && a.googleConnected,
		Playlists: []GooglePlaylistView{},
	}
	if v.Connected {
		v.Email = a.googleEmail
		v.Playlists = append(v.Playlists, a.googleLists...)
		v.SyncError = a.syncErr
		v.SyncRevision = a.syncRevision
	}
	return v
}

// signedOutLocked dimentica l'account collegato (accesso scaduto o
// scollegamento). Va chiamata con il lock acquisito.
func (a *App) signedOutLocked() {
	a.googleConnected = false
	a.googleEmail = ""
	a.googleLists = nil
	a.googleListsAt = time.Time{}
	a.syncErr = ""
	a.persistStateLocked()
}

// fetchGoogleLists rilegge le playlist dell'account e le tiene in memoria.
func (a *App) fetchGoogleLists(ctx context.Context) error {
	list, err := a.youtubeClient().Playlists(ctx)
	if err != nil {
		return err
	}
	views := make([]GooglePlaylistView, 0, len(list))
	for _, p := range list {
		views = append(views, GooglePlaylistView{
			ID:      p.ID,
			Title:   p.Title,
			URL:     youtube.PlaylistURL(p.ID),
			Count:   p.Count,
			Privacy: p.Privacy,
		})
	}
	a.mu.Lock()
	a.googleLists = views
	a.googleListsAt = time.Now()
	a.mu.Unlock()
	return nil
}

// refreshGoogleListsInBackground legge le playlist dell'account all'avvio
// (se collegato) e notifica la UI. Senza rete non segnala nulla: ci si
// riprova al ritorno sulla finestra (RefreshGooglePlaylists).
func (a *App) refreshGoogleListsInBackground() {
	_ = a.safely("playlist dell'account Google", func() error {
		if a.googleReady() != "" {
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), googleCallTimeout)
		defer cancel()
		if err := a.fetchGoogleLists(ctx); err != nil {
			msg := a.googleError(err)
			if !errors.Is(err, google.ErrSignedOut) {
				a.mu.Lock()
				a.addLogLocked(LogError, "Impossibile leggere le playlist dell'account Google: "+msg)
				a.mu.Unlock()
			}
		}
		// Impostazioni delle playlist cambiate da un altro dispositivo.
		a.syncSettings(ctx)
		a.emit(EventGoogleChanged, a.snapshotLocked())
		return nil
	})
}

// youtubeClient è il client della YouTube Data API con le credenziali
// dell'account collegato (a.google non nil).
func (a *App) youtubeClient() *youtube.Client {
	return &youtube.Client{Token: a.google.Token, Invalidate: a.google.Invalidate}
}

// googleReady indica se c'è un account collegato utilizzabile; altrimenti
// restituisce il messaggio da mostrare.
func (a *App) googleReady() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	v := a.googleViewLocked()
	if !v.Available {
		return "Accesso a Google non disponibile in questa versione dell'app."
	}
	if !v.Connected {
		return "Collega prima l'account Google nelle Impostazioni (scheda Download)."
	}
	return ""
}

// googleError trasforma un errore dell'accesso o dell'API nel messaggio da
// mostrare. Un accesso non più valido scollega l'account, così la UI propone
// di ricollegarlo.
func (a *App) googleError(err error) string {
	if errors.Is(err, google.ErrSignedOut) {
		a.mu.Lock()
		a.signedOutLocked()
		a.addLogLocked(LogError, msgGoogleSignedOut)
		a.mu.Unlock()
		return msgGoogleSignedOut
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "YouTube non ha risposto in tempo: riprova."
	}
	return capitalize(err.Error()) + "."
}

// GoogleSignIn collega l'account Google: su desktop apre la pagina di accesso
// nel browser e attende che l'utente la completi (annullabile con Cancel), su
// Android mostra la schermata di Google Play Services.
func (a *App) GoogleSignIn() ActionResponse {
	a.mu.Lock()
	available := a.googleViewLocked().Available
	a.mu.Unlock()
	if !available {
		return ActionResponse{OK: false, Message: "Accesso a Google non disponibile in questa versione dell'app.", State: a.snapshotLocked()}
	}

	ctx, endOp := a.beginCancelable()
	defer endOp()

	if err := a.google.SignIn(ctx); err != nil {
		if ctx.Err() != nil {
			return ActionResponse{OK: false, Message: "Accesso a Google annullato.", State: a.snapshotLocked()}
		}
		msg := "Accesso a Google non riuscito: " + err.Error() + "."
		a.mu.Lock()
		a.addLogLocked(LogError, msg)
		state := a.snapshot()
		a.mu.Unlock()
		return ActionResponse{OK: false, Message: msg, State: state}
	}

	// L'email serve solo a mostrare quale account è collegato, e le playlist a
	// proporle subito nella scelta del download: se non si riesce a leggerle
	// l'account resta comunque collegato.
	callCtx, cancel := context.WithTimeout(ctx, googleCallTimeout)
	defer cancel()
	email := ""
	if token, err := a.google.Token(callCtx); err == nil {
		email, _ = google.UserEmail(callCtx, token)
	}

	a.mu.Lock()
	a.googleConnected = true
	a.googleEmail = email
	a.googleLists = nil
	a.persistStateLocked()
	a.mu.Unlock()
	listErr := a.fetchGoogleLists(callCtx)
	// Impostazioni delle playlist: quelle dell'account, se ci sono e più recenti.
	a.syncSettings(callCtx)

	msg := "Account Google collegato."
	if email != "" {
		msg = fmt.Sprintf("Account Google collegato: %s.", email)
	}
	if listErr != nil {
		// L'account è collegato, ma le playlist non si leggono: lo si dice
		// subito, invece di lasciare vuota la scelta della playlist.
		listMsg := "Impossibile leggere le playlist: " + a.googleError(listErr)
		a.mu.Lock()
		defer a.mu.Unlock()
		a.addLogLocked(LogError, listMsg)
		return ActionResponse{OK: false, Message: msg + " " + listMsg, State: a.snapshot()}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.addLogLocked(LogSuccess, msg)
	return ActionResponse{OK: true, Message: msg, State: a.snapshot()}
}

// GoogleSignOut scollega l'account Google e revoca il permesso concesso
// all'app. L'account si scollega anche se la revoca non riesce (es. offline).
func (a *App) GoogleSignOut() ActionResponse {
	var err error
	if a.google != nil {
		ctx, cancel := context.WithTimeout(context.Background(), googleCallTimeout)
		err = a.google.SignOut(ctx)
		cancel()
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.signedOutLocked()
	msg := "Account Google scollegato."
	if err != nil && !errors.Is(err, google.ErrSignedOut) {
		msg = "Account Google scollegato, ma la revoca del permesso su Google non è riuscita: puoi toglierlo da myaccount.google.com/permissions."
	}
	a.addLogLocked(LogInfo, msg)
	return ActionResponse{OK: true, Message: msg, State: a.snapshot()}
}

// GooglePlaylists rilegge e restituisce le playlist dell'account Google
// collegato (campo GooglePlaylists della risposta, e Google.Playlists dello
// stato).
func (a *App) GooglePlaylists() ActionResponse {
	if msg := a.googleReady(); msg != "" {
		return ActionResponse{OK: false, Message: msg, State: a.snapshotLocked()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), googleCallTimeout)
	defer cancel()
	if err := a.fetchGoogleLists(ctx); err != nil {
		msg := "Impossibile leggere le playlist: " + a.googleError(err)
		return ActionResponse{OK: false, Message: msg, State: a.snapshotLocked()}
	}
	state := a.snapshotLocked()
	return ActionResponse{OK: true, State: state, GooglePlaylists: state.Google.Playlists}
}

// RefreshGooglePlaylists va chiamata al ritorno sulla finestra: confronta
// sempre le impostazioni con quelle sull'account (cambiate da un altro
// dispositivo) e rilegge le playlist dell'account solo se l'ultima lettura
// risale a più di googleListsMaxAge (quota di YouTube), così compaiono anche
// quelle create nel frattempo. Senza account collegato non fa nulla. Un
// errore di rete non si segnala (si riproverà); un accesso scaduto scollega
// l'account.
func (a *App) RefreshGooglePlaylists() ActionResponse {
	if a.googleReady() != "" {
		return ActionResponse{OK: true, State: a.snapshotLocked()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), googleCallTimeout)
	defer cancel()
	a.syncSettings(ctx)

	a.mu.Lock()
	fresh := time.Since(a.googleListsAt) < googleListsMaxAge
	a.mu.Unlock()
	if fresh {
		return ActionResponse{OK: true, State: a.snapshotLocked()}
	}
	err := a.fetchGoogleLists(ctx)
	if err != nil && errors.Is(err, google.ErrSignedOut) {
		return ActionResponse{OK: false, Message: a.googleError(err), State: a.snapshotLocked()}
	}
	return ActionResponse{OK: true, State: a.snapshotLocked()}
}

// DownloadGooglePlaylist scarica la playlist id dell'account Google collegato,
// come DownloadPlaylist per una playlist salvata. Se tra quelle salvate ce n'è
// una con lo stesso ID e «Svuota dopo il download», vale anche qui.
func (a *App) DownloadGooglePlaylist(id string) ActionResponse {
	src, msg := a.googleSource(id)
	if msg != "" {
		return ActionResponse{OK: false, Message: msg, State: a.snapshotLocked()}
	}
	opCtx, endOp := a.beginCancelable()
	defer endOp()
	resp, _ := a.download(opCtx, src)
	return resp
}

// DownloadGooglePlaylistAndProcess è DownloadAndProcess per una playlist
// dell'account Google collegato (vedi DownloadGooglePlaylist).
func (a *App) DownloadGooglePlaylistAndProcess(id string) ActionResponse {
	src, msg := a.googleSource(id)
	if msg != "" {
		return ActionResponse{OK: false, Message: msg, State: a.snapshotLocked()}
	}
	return a.downloadAndProcess(func(opCtx context.Context) (ActionResponse, bool) {
		return a.download(opCtx, src)
	})
}

// googleSource è la sorgente di download della playlist id dell'account.
func (a *App) googleSource(id string) (downloadSource, string) {
	if msg := a.googleReady(); msg != "" {
		return downloadSource{}, msg
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	src := downloadSource{}
	for _, p := range a.googleLists {
		if p.ID == id {
			src.url, src.playlist = p.URL, p.Title
			break
		}
	}
	if src.url == "" {
		return downloadSource{}, "Playlist non trovata sull'account Google: riapri l'elenco e riprova."
	}
	src.prefs = a.playlistPrefs[googleKey(id)]
	return src, ""
}

// AddLinkToPlaylist aggiunge il video del link alla playlist playlistID
// dell'account collegato (title è il nome della playlist, per i messaggi). Se
// il video c'è già non lo aggiunge una seconda volta.
func (a *App) AddLinkToPlaylist(link, playlistID, title string) ActionResponse {
	if msg := a.googleReady(); msg != "" {
		return ActionResponse{OK: false, Message: msg, State: a.snapshotLocked()}
	}
	videoID := youtube.VideoID(link)
	if videoID == "" {
		return ActionResponse{OK: false, Message: "Il link non è quello di un video di YouTube.", State: a.snapshotLocked()}
	}

	ctx, cancel := context.WithTimeout(context.Background(), googleCallTimeout)
	defer cancel()
	c := a.youtubeClient()
	items, err := c.Items(ctx, playlistID)
	if err == nil {
		for _, it := range items {
			if it.VideoID == videoID {
				return ActionResponse{OK: true, Message: fmt.Sprintf("Il video è già nella playlist %q.", title), State: a.snapshotLocked()}
			}
		}
		err = c.Insert(ctx, playlistID, videoID)
	}
	if err != nil {
		msg := "Video non aggiunto alla playlist: " + a.googleError(err)
		a.mu.Lock()
		a.addLogLocked(LogError, msg)
		state := a.snapshot()
		a.mu.Unlock()
		return ActionResponse{OK: false, Message: msg, State: state}
	}

	msg := fmt.Sprintf("Video aggiunto alla playlist %q.", title)
	a.mu.Lock()
	a.addLogLocked(LogSuccess, msg)
	state := a.snapshot()
	a.mu.Unlock()
	return ActionResponse{OK: true, Message: msg, State: state}
}

// removeFromPlaylist toglie dalla playlist di src, su YouTube, gli elementi
// dei video videoIDs appena scaricati (tutte le copie, se un video compare più
// volte). Restituisce una breve nota per la UI e se la rimozione è fallita
// (anche solo in parte). Non usa il context dell'operazione: anche dopo un
// annullamento i brani già scaricati vanno tolti, altrimenti il prossimo
// download li riscaricherebbe.
func (a *App) removeFromPlaylist(src downloadSource, videoIDs []string) (note string, failed bool) {
	fail := func(msg string) (string, bool) {
		a.mu.Lock()
		a.addLogLocked(LogError, fmt.Sprintf("Playlist %q non svuotata: %s", src.playlist, msg))
		a.mu.Unlock()
		return "Playlist su YouTube non svuotata: " + msg, true
	}

	if msg := a.googleReady(); msg != "" {
		return fail(msg)
	}
	listID := youtube.PlaylistID(src.url)
	if listID == "" {
		return fail("il link non contiene l'ID della playlist (parametro list=).")
	}

	ctx, cancel := context.WithTimeout(context.Background(), googleRemoveTimeout)
	defer cancel()
	c := a.youtubeClient()
	items, err := c.Items(ctx, listID)
	if err != nil {
		return fail(a.googleError(err))
	}

	want := make(map[string]bool, len(videoIDs))
	for _, id := range videoIDs {
		want[id] = true
	}
	// Si contano i video, non gli elementi: un video presente due volte nella
	// playlist è un solo brano scaricato. Un video è tolto se lo sono tutte le
	// sue copie.
	deleted, missedSet := map[string]bool{}, map[string]bool{}
	var lastErr error
	for _, it := range items {
		if !want[it.VideoID] {
			continue
		}
		// Dopo un errore di quota o di accesso è inutile continuare.
		if lastErr != nil && (youtube.IsQuota(lastErr) || errors.Is(lastErr, google.ErrSignedOut)) {
			missedSet[it.VideoID] = true
			continue
		}
		if err := c.Delete(ctx, it.ID); err != nil {
			lastErr = err
			missedSet[it.VideoID] = true
			continue
		}
		deleted[it.VideoID] = true
	}
	removed, missed := 0, len(missedSet)
	for id := range deleted {
		if !missedSet[id] {
			removed++
		}
	}

	if lastErr != nil {
		msg := a.googleError(lastErr)
		if removed > 0 {
			msg = fmt.Sprintf("tolti %d brani, %d no: %s", removed, missed, msg)
		}
		return fail(msg)
	}
	if removed == 0 {
		return "", false
	}
	a.mu.Lock()
	a.addLogLocked(LogSuccess, fmt.Sprintf("Playlist %q: tolti da YouTube %d brani scaricati.", src.playlist, removed))
	a.mu.Unlock()
	if removed == 1 {
		return "1 brano tolto dalla playlist su YouTube.", false
	}
	return fmt.Sprintf("%d brani tolti dalla playlist su YouTube.", removed), false
}

// afterDownload esegue, sui brani videoIDs appena scaricati da src, ciò che
// chiedono le impostazioni della playlist: niente, toglierli dalla playlist
// su YouTube o aggiungerli a un'altra playlist dell'account. Restituisce la
// nota per la UI ("" se non c'era nulla da fare) e se è fallito.
func (a *App) afterDownload(src downloadSource, videoIDs []string) (note string, failed bool) {
	if len(videoIDs) == 0 {
		return "", false
	}
	switch src.prefs.AfterDownload {
	case playlist.AfterRemove:
		return a.removeFromPlaylist(src, videoIDs)
	case playlist.AfterCopy:
		note, failed = a.copyToPlaylist(src, videoIDs)
		// Spostamento: dall'origine si tolgono solo se sono tutti nella
		// destinazione, altrimenti un brano potrebbe sparire da entrambe.
		if failed || !src.prefs.MoveOnCopy {
			return note, failed
		}
		removeNote, removeFailed := a.removeFromPlaylist(src, videoIDs)
		return strings.TrimSpace(note + " " + removeNote), removeFailed
	}
	return "", false
}

// copyToPlaylist aggiunge i video videoIDs, nell'ordine dato, in fondo alla
// playlist di destinazione delle impostazioni di src, saltando quelli che ci
// sono già. Come removeFromPlaylist non usa il context dell'operazione.
func (a *App) copyToPlaylist(src downloadSource, videoIDs []string) (note string, failed bool) {
	target, title := src.prefs.CopyTo, src.prefs.CopyToTitle
	if title == "" {
		title = target
	}
	fail := func(msg string) (string, bool) {
		a.mu.Lock()
		a.addLogLocked(LogError, fmt.Sprintf("Brani di %s non aggiunti alla playlist %q: %s", src.what(), title, msg))
		a.mu.Unlock()
		return fmt.Sprintf("Brani non aggiunti alla playlist %q: %s", title, msg), true
	}
	if msg := a.googleReady(); msg != "" {
		return fail(msg)
	}

	ctx, cancel := context.WithTimeout(context.Background(), googleRemoveTimeout)
	defer cancel()
	c := a.youtubeClient()
	items, err := c.Items(ctx, target)
	if err != nil {
		return fail(a.googleError(err))
	}
	present := make(map[string]bool, len(items))
	for _, it := range items {
		present[it.VideoID] = true
	}
	added := 0
	for _, id := range videoIDs {
		if present[id] {
			continue
		}
		if err := c.Insert(ctx, target, id); err != nil {
			msg := a.googleError(err)
			if added > 0 {
				msg = fmt.Sprintf("aggiunti %d brani, poi: %s", added, msg)
			}
			return fail(msg)
		}
		present[id] = true
		added++
	}
	if added == 0 {
		return "", false
	}
	a.mu.Lock()
	a.addLogLocked(LogSuccess, fmt.Sprintf("Aggiunti %d brani scaricati alla playlist %q.", added, title))
	a.mu.Unlock()
	if added == 1 {
		return fmt.Sprintf("1 brano aggiunto alla playlist %q.", title), false
	}
	return fmt.Sprintf("%d brani aggiunti alla playlist %q.", added, title), false
}

// EmptyGooglePlaylist toglie da YouTube tutti i brani della playlist id
// dell'account collegato. È un'operazione lunga e annullabile (Cancel), con
// l'avanzamento su EventProcessProgress; i brani già tolti restano tolti.
// Azione distruttiva: la UI la conferma prima.
func (a *App) EmptyGooglePlaylist(id string) ActionResponse {
	if msg := a.googleReady(); msg != "" {
		return ActionResponse{OK: false, Message: msg, State: a.snapshotLocked()}
	}
	a.mu.Lock()
	title := ""
	for _, p := range a.googleLists {
		if p.ID == id {
			title = p.Title
		}
	}
	a.mu.Unlock()
	if title == "" {
		return ActionResponse{OK: false, Message: "Si possono svuotare solo le playlist del tuo account.", State: a.snapshotLocked()}
	}

	opCtx, endOp := a.beginCancelable()
	defer endOp()
	c := a.youtubeClient()
	items, err := c.Items(opCtx, id)
	removed := 0
	if err == nil {
		a.emit(EventProcessProgress, ProgressEvent{Done: 0, Total: len(items)})
		for _, it := range items {
			if opCtx.Err() != nil {
				break
			}
			if err = c.Delete(opCtx, it.ID); err != nil {
				break
			}
			removed++
			a.emit(EventProcessProgress, ProgressEvent{Done: removed, Total: len(items)})
		}
	}
	canceled := opCtx.Err() != nil

	// Il numero di brani della playlist è cambiato.
	listCtx, cancel := context.WithTimeout(context.Background(), googleCallTimeout)
	_ = a.fetchGoogleLists(listCtx)
	cancel()

	switch {
	case canceled:
		msg := fmt.Sprintf("Svuotamento della playlist %q annullato: tolti %d brani.", title, removed)
		a.mu.Lock()
		defer a.mu.Unlock()
		a.addLogLocked(LogInfo, msg)
		return ActionResponse{OK: true, Message: msg, State: a.snapshot()}
	case err != nil:
		why := a.googleError(err)
		msg := fmt.Sprintf("Playlist %q non svuotata: %s", title, why)
		if removed > 0 {
			msg = fmt.Sprintf("Playlist %q svuotata solo in parte (tolti %d brani): %s", title, removed, why)
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		a.addLogLocked(LogError, msg)
		return ActionResponse{OK: false, Message: msg, State: a.snapshot()}
	}
	msg := fmt.Sprintf("Playlist %q svuotata: tolti %d brani.", title, removed)
	if removed == 0 {
		msg = fmt.Sprintf("La playlist %q era già vuota.", title)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.addLogLocked(LogSuccess, msg)
	return ActionResponse{OK: true, Message: msg, State: a.snapshot()}
}

// capitalize rende maiuscola la prima lettera (i messaggi d'errore dei
// package sono in minuscolo, come d'uso in Go).
func capitalize(s string) string {
	for i, r := range s {
		if r >= 'a' && r <= 'z' {
			return s[:i] + string(r-'a'+'A') + s[i+1:]
		}
		return s
	}
	return s
}
