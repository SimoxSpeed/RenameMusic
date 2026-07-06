package parser

import "testing"

func TestTagArtistAndTitleSimple(t *testing.T) {
	name := "Artist - Title.mp3"

	if got := TagArtist(name); got != "Artist" {
		t.Fatalf("TagArtist() = %q, want %q", got, "Artist")
	}
	if got := TagTitle(name); got != "Title" {
		t.Fatalf("TagTitle() = %q, want %q", got, "Title")
	}
}

func TestTagArtistAndTitleRemix(t *testing.T) {
	name := "Artist & Other x Third - Song ft Guest x Another (Remixer Remix).mp3"

	if got := TagArtist(name); got != "Remixer, Artist, Other, Third, Guest, Another" {
		t.Fatalf("TagArtist() = %q", got)
	}
	if got := TagTitle(name); got != "Song ft Guest, Another (Remixer Remix)" {
		t.Fatalf("TagTitle() = %q", got)
	}
}

func TestTagTitleVIPMatchesJavaSpacing(t *testing.T) {
	name := "Artist - Song ft A x B VIP .mp3"

	if got := TagTitle(name); got != "Song ft A, B VIP" {
		t.Fatalf("TagTitle() = %q", got)
	}
}

func TestUnknownArtistAndTitle(t *testing.T) {
	name := "Loose Track.mp3"

	if got := TagArtist(name); got != UnknownArtist {
		t.Fatalf("TagArtist() = %q, want %q", got, UnknownArtist)
	}
	if got := TagTitle(name); got != UnknownTitle {
		t.Fatalf("TagTitle() = %q, want %q", got, UnknownTitle)
	}
}
