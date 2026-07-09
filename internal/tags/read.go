package tags

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"unicode/utf16"
)

// maxTagBytes limita quanto del tag ID3 viene letto in memoria: TIT2/TPE1
// compaiono sempre tra i primi frame di un tag scritto da WriteMP3Tags, e
// anche nei tag di terzi con copertina incorporata restano ben prima
// dell'eventuale APIC. Il limite evita letture spropositate su un tag con
// dimensione dichiarata anomala, senza dover mai caricare l'intero file audio.
const maxTagBytes = 2 << 20 // 2 MiB

// ReadMP3Tags legge titolo (TIT2) e artista (TPE1) da un file MP3 con tag
// ID3v2.3, come quelli prodotti da WriteMP3Tags. Legge da disco solo l'header
// ID3 e il tag dichiarato (non l'intero file, che per un mp3 può pesare
// diversi MB): è un parser INDIPENDENTE dallo scrittore, usato per verificare
// (round-trip) che ciò che scriviamo sia effettivamente rileggibile, non per
// interpretare tag arbitrari di terzi (di cui gestisce comunque le codifiche
// di testo più comuni).
func ReadMP3Tags(path string) (title string, artist string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	header := make([]byte, 10)
	if _, err := io.ReadFull(f, header); err != nil {
		return "", "", fmt.Errorf("header ID3 assente")
	}
	if string(header[:3]) != "ID3" {
		return "", "", fmt.Errorf("header ID3 assente")
	}

	tagSize := syncSafeToInt(header[6:10])
	if tagSize > maxTagBytes {
		tagSize = maxTagBytes
	}
	body := make([]byte, tagSize)
	n, err := io.ReadFull(f, body)
	if err != nil && err != io.ErrUnexpectedEOF {
		return "", "", err
	}

	frames := parseID3v23FrameBody(body[:n])
	return frames["TIT2"], frames["TPE1"], nil
}

// parseID3v23FrameBody estrae i frame di testo dal corpo di un tag ID3v2.3
// (i byte subito dopo l'header di 10 byte, fino alla dimensione dichiarata).
func parseID3v23FrameBody(body []byte) map[string]string {
	frames := make(map[string]string)
	pos := 0
	end := len(body)
	// Ogni frame ID3v2.3: 4 byte ID + 4 byte size (big-endian, NON syncsafe) +
	// 2 byte flags + payload. Uno zero come primo byte dell'ID indica padding.
	for pos+10 <= end {
		id := string(body[pos : pos+4])
		if id[0] == 0 {
			break
		}
		size := int(binary.BigEndian.Uint32(body[pos+4 : pos+8]))
		pos += 10
		if size < 0 || pos+size > end {
			break
		}
		if id == "TIT2" || id == "TPE1" {
			frames[id] = decodeTextFrame(body[pos : pos+size])
		}
		pos += size
	}
	return frames
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
