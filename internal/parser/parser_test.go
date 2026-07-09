package parser

import "testing"

// noExceptions: nessun nome d'arte protetto (i test qui sotto non ne usano).
var noExceptions []string

func TestTagArtistAndTitleSimple(t *testing.T) {
	name := "Artist - Title.mp3"

	if got := TagArtist(name, noExceptions); got != "Artist" {
		t.Fatalf("TagArtist() = %q, want %q", got, "Artist")
	}
	if got := TagTitle(name, noExceptions); got != "Title" {
		t.Fatalf("TagTitle() = %q, want %q", got, "Title")
	}
}

func TestTagArtistAndTitleRemix(t *testing.T) {
	name := "Artist & Other x Third - Song ft Guest x Another (Remixer Remix).mp3"

	if got := TagArtist(name, noExceptions); got != "Remixer, Artist, Other, Third, Guest, Another" {
		t.Fatalf("TagArtist() = %q", got)
	}
	if got := TagTitle(name, noExceptions); got != "Song ft Guest, Another (Remixer Remix)" {
		t.Fatalf("TagTitle() = %q", got)
	}
}

func TestTagTitleVIPMatchesJavaSpacing(t *testing.T) {
	name := "Artist - Song ft A x B VIP .mp3"

	if got := TagTitle(name, noExceptions); got != "Song ft A, B VIP" {
		t.Fatalf("TagTitle() = %q", got)
	}
}

func TestTagVIPInTitleNotTreatedAsSuffix(t *testing.T) {
	name := "File example - Mock VIP ft Jabra.mp3"

	if got := TagTitle(name, noExceptions); got != "Mock VIP ft Jabra" {
		t.Fatalf("TagTitle() = %q, want %q", got, "Mock VIP ft Jabra")
	}
	if got := TagArtist(name, noExceptions); got != "File example, Jabra" {
		t.Fatalf("TagArtist() = %q, want %q", got, "File example, Jabra")
	}
}

func TestUnknownArtistAndTitle(t *testing.T) {
	name := "Loose Track.mp3"

	if got := TagArtist(name, noExceptions); got != UnknownArtist {
		t.Fatalf("TagArtist() = %q, want %q", got, UnknownArtist)
	}
	if got := TagTitle(name, noExceptions); got != UnknownTitle {
		t.Fatalf("TagTitle() = %q, want %q", got, UnknownTitle)
	}
}

// Un nome d'arte con " & " presente tra le eccezioni NON va spezzato in due
// artisti; senza eccezioni, lo stesso nome viene invece separato.
func TestTagArtistHonorsExceptions(t *testing.T) {
	name := "Jkyl & Hyde - Song.mp3"

	if got := TagArtist(name, []string{"Jkyl & Hyde"}); got != "Jkyl & Hyde" {
		t.Fatalf("TagArtist() con eccezione = %q, want %q", got, "Jkyl & Hyde")
	}
	if got := TagArtist(name, noExceptions); got != "Jkyl, Hyde" {
		t.Fatalf("TagArtist() senza eccezione = %q, want %q", got, "Jkyl, Hyde")
	}
}

func TestReplaceWithCommaExceptions(t *testing.T) {
	got := ReplaceWithComma("A & Jkyl & Hyde & B", []string{"Jkyl & Hyde"})
	if got != "A, Jkyl & Hyde, B" {
		t.Fatalf("ReplaceWithComma() = %q, want %q", got, "A, Jkyl & Hyde, B")
	}
}
