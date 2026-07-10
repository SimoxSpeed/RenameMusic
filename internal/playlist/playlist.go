package playlist

// Playlist associa un nome scelto dall'utente al link di una playlist
// YouTube, così la UI può mostrare un select leggibile invece del link grezzo.
type Playlist struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}
