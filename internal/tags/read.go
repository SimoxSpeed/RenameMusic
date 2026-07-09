package tags

import (
	"encoding/binary"
	"fmt"
	"os"
	"unicode/utf16"
)

// ReadMP3Tags legge titolo (TIT2) e artista (TPE1) da un file MP3 con tag
// ID3v2.3, come quelli prodotti da WriteMP3Tags. È un parser INDIPENDENTE dallo
// scrittore: serve a verificare (round-trip) che ciò che scriviamo sia
// effettivamente rileggibile, non a interpretare tag arbitrari di terzi (di cui
// gestisce comunque le codifiche di testo più comuni).
func ReadMP3Tags(path string) (title string, artist string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	frames, err := parseID3v23Frames(data)
	if err != nil {
		return "", "", err
	}
	return frames["TIT2"], frames["TPE1"], nil
}

// parseID3v23Frames estrae i frame di testo da un tag ID3v2.3 in testa ai dati.
func parseID3v23Frames(data []byte) (map[string]string, error) {
	if len(data) < 10 || string(data[:3]) != "ID3" {
		return nil, fmt.Errorf("header ID3 assente")
	}

	end := 10 + syncSafeToInt(data[6:10])
	if end > len(data) {
		end = len(data)
	}

	frames := make(map[string]string)
	pos := 10
	// Ogni frame ID3v2.3: 4 byte ID + 4 byte size (big-endian, NON syncsafe) +
	// 2 byte flags + payload. Uno zero come primo byte dell'ID indica padding.
	for pos+10 <= end {
		id := string(data[pos : pos+4])
		if id[0] == 0 {
			break
		}
		size := int(binary.BigEndian.Uint32(data[pos+4 : pos+8]))
		pos += 10
		if size < 0 || pos+size > end {
			break
		}
		if id == "TIT2" || id == "TPE1" {
			frames[id] = decodeTextFrame(data[pos : pos+size])
		}
		pos += size
	}
	return frames, nil
}

// decodeTextFrame decodifica il payload di un frame di testo ID3v2.3 in base al
// byte di encoding iniziale. Gestisce ISO-8859-1 (0x00) e UTF-16 con BOM (0x01),
// rimuovendo eventuale terminatore NUL finale.
func decodeTextFrame(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	enc, rest := payload[0], payload[1:]

	switch enc {
	case 0x00: // ISO-8859-1
		for len(rest) > 0 && rest[len(rest)-1] == 0 {
			rest = rest[:len(rest)-1]
		}
		runes := make([]rune, len(rest))
		for i, b := range rest {
			runes[i] = rune(b)
		}
		return string(runes)

	case 0x01: // UTF-16 con BOM
		if len(rest) < 2 {
			return ""
		}
		order := binary.ByteOrder(binary.LittleEndian)
		if rest[0] == 0xFE && rest[1] == 0xFF {
			order = binary.BigEndian
		}
		rest = rest[2:]

		units := make([]uint16, 0, len(rest)/2)
		for i := 0; i+1 < len(rest); i += 2 {
			units = append(units, order.Uint16(rest[i:]))
		}
		for len(units) > 0 && units[len(units)-1] == 0 {
			units = units[:len(units)-1]
		}
		return string(utf16.Decode(units))

	default:
		return ""
	}
}
