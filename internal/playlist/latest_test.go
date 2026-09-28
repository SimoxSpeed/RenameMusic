package playlist

import "testing"

func TestNewerVersion(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"2026.08.19", "2026.07.04", true},
		{"2026.07.04", "2026.07.04", false},
		{"2026.07.04", "2026.08.19", false},
		{"2026.07.04.1", "2026.07.04", true},
		{"2026.07.04", "2026.07.04.1", false},
		{"2026.10.01", "2026.9.30", true},
		{"2026.08.19", "", true},
		{"2026.08.19", "sconosciuta", true},
		{"", "2026.07.04", false},
		{"v2026", "2026.07.04", false},
	}
	for _, c := range cases {
		if got := NewerVersion(c.latest, c.current); got != c.want {
			t.Errorf("NewerVersion(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}
