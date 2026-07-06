package tags

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteMP3TagsWritesID3HeaderAndStripsOldTags(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "track.mp3")
	oldTag := append([]byte{'I', 'D', '3', 3, 0, 0, 0, 0, 0, 0}, []byte("audio")...)
	if err := os.WriteFile(path, oldTag, 0644); err != nil {
		t.Fatal(err)
	}

	if err := WriteMP3Tags(path, "Title", "Artist"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data[:3]) != "ID3" {
		t.Fatalf("missing ID3 header: %q", data[:3])
	}
	size := syncSafeToInt(data[6:10])
	if size <= 0 {
		t.Fatalf("invalid ID3 size: %d", size)
	}
	if got := string(data[10:14]); got != "TIT2" {
		t.Fatalf("first frame = %q, want TIT2", got)
	}
	if got := string(data[10+10+1+2+len("Title")*2 : 10+10+1+2+len("Title")*2+4]); got != "TPE1" {
		t.Fatalf("second frame = %q, want TPE1", got)
	}
	if string(data[len(data)-5:]) != "audio" {
		t.Fatal("audio payload was not preserved")
	}
}
