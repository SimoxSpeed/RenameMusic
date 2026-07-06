package rules

import "testing"

func TestNormalizeFileBaseKeepsJavaRuleOrder(t *testing.T) {
	input := "Artist X Guest - Title feat. Other (VIP) [cut] Re-Crank tha Supreme Prod.   z"
	want := "Artist x Guest - Title ft Other VIP Remix thasup prod. z"

	if got := NormalizeFileBase(input); got != want {
		t.Fatalf("NormalizeFileBase() = %q, want %q", got, want)
	}
}

func TestNormalizeFileBaseFtParenthesis(t *testing.T) {
	input := "Artist - Title (ft Guest)"
	want := "Artist - Title ft Guest"

	if got := NormalizeFileBase(input); got != want {
		t.Fatalf("NormalizeFileBase() = %q, want %q", got, want)
	}
}

func TestSupportedExtensionsAreCaseSensitiveLikeJava(t *testing.T) {
	if !IsSupportedExtension("mp3") {
		t.Fatal("mp3 should be supported")
	}
	if IsSupportedExtension("MP3") {
		t.Fatal("MP3 should not be supported")
	}
}
