package playlist

// Playlist associa un nome scelto dall'utente al link di una playlist
// YouTube, così la UI può mostrare un select leggibile invece del link grezzo.
type Playlist struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Cosa fare, a download finito, dei brani scaricati da una playlist
// (Prefs.AfterDownload). Servono l'account Google collegato e, per
// AfterRemove, che la playlist sia dell'account.
const (
	AfterNothing = ""
	AfterRemove  = "remove" // tolti dalla playlist su YouTube (la playlist fa da coda)
	AfterCopy    = "copy"   // aggiunti a un'altra playlist dell'account (CopyTo)
)

// Prefs sono le impostazioni di una playlist della scelta del download, sia
// dell'account Google sia salvata a mano. Si conservano per chiave (vedi
// core.playlistKey): così una playlist salvata che è anche dell'account ha le
// stesse impostazioni nei due elenchi.
type Prefs struct {
	// Hidden: la playlist non compare nella scelta del download.
	Hidden bool `json:"hidden,omitempty"`
	// AfterDownload: AfterNothing, AfterRemove o AfterCopy.
	AfterDownload string `json:"afterDownload,omitempty"`
	// CopyTo / CopyToTitle: ID e nome della playlist di destinazione di
	// AfterCopy (il nome serve solo ai messaggi).
	CopyTo      string `json:"copyTo,omitempty"`
	CopyToTitle string `json:"copyToTitle,omitempty"`
	// MoveOnCopy: con AfterCopy, i brani aggiunti a CopyTo si tolgono poi
	// dalla playlist di origine (si spostano). Solo per le playlist dell'account.
	MoveOnCopy bool `json:"moveOnCopy,omitempty"`
}

// IsZero indica impostazioni tutte predefinite (non serve salvarle).
func (p Prefs) IsZero() bool { return p == Prefs{} }
