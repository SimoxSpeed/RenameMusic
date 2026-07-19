package rules

import "testing"

func TestNormalizeFileBaseKeepsJavaRuleOrder(t *testing.T) {
	input := "Artist X Guest - Title feat. Other (VIP) [cut] Re-Crank tha Supreme Prod.   z"
	want := "Artist x Guest - Title ft Other VIP Remix thasup prod. z"

	if got := FactoryConfig().NormalizeFileBase(input); got != want {
		t.Fatalf("NormalizeFileBase() = %q, want %q", got, want)
	}
}

func TestScopedReplacementArtistOnly(t *testing.T) {
	cfg := FactoryConfig() // " X " -> " x " ha Scope ScopeArtist

	// " X " nella parte artista (prima di " - ") va convertita.
	if got := cfg.NormalizeFileBase("A X B - Titolo"); got != "A x B - Titolo" {
		t.Fatalf("artista: got %q", got)
	}
	// " X " nella parte titolo (dopo " - ") NON va convertita.
	if got := cfg.NormalizeFileBase("Artista - A X B"); got != "Artista - A X B" {
		t.Fatalf("titolo: got %q", got)
	}
	// Senza separatore " - " non si sa distinguere le parti: nessuna conversione.
	if got := cfg.NormalizeFileBase("A X B"); got != "A X B" {
		t.Fatalf("senza separatore: got %q", got)
	}
}

func TestScopedReplacementTitleOnly(t *testing.T) {
	cfg := FactoryConfig()
	cfg.Replacements = []Replacement{{From: " X ", To: " x ", Scope: ScopeTitle}}

	if got := cfg.NormalizeFileBase("A X B - C X D"); got != "A X B - C x D" {
		t.Fatalf("solo titolo: got %q", got)
	}
}

func TestNormalizeFileBaseFtParenthesis(t *testing.T) {
	input := "Artist - Title (ft Guest)"
	want := "Artist - Title ft Guest"

	if got := FactoryConfig().NormalizeFileBase(input); got != want {
		t.Fatalf("NormalizeFileBase() = %q, want %q", got, want)
	}
}

func TestNormalizeConvertsTypographicDashAndW(t *testing.T) {
	cfg := FactoryConfig()
	if got := cfg.NormalizeFileBase("Artista – Titolo"); got != "Artista - Titolo" {
		t.Fatalf("en dash: got %q", got)
	}
	if got := cfg.NormalizeFileBase("Artista — Titolo"); got != "Artista - Titolo" {
		t.Fatalf("em dash: got %q", got)
	}
	if got := cfg.NormalizeFileBase("Song w/ Guest"); got != "Song ft Guest" {
		t.Fatalf("w/: got %q", got)
	}
	if got := cfg.NormalizeFileBase("w/o warning"); got != "w/o warning" {
		t.Fatalf("w/o non deve cambiare: got %q", got)
	}
}

func TestSupportedExtensionsAreCaseSensitiveLikeJava(t *testing.T) {
	cfg := FactoryConfig()
	if !cfg.IsSupportedExtension("mp3") {
		t.Fatal("mp3 should be supported")
	}
	if cfg.IsSupportedExtension("MP3") {
		t.Fatal("MP3 should not be supported")
	}
}
