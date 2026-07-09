package tags

import (
	"bytes"
	"encoding/binary"
	"os"
	"unicode/utf16"
)

func WriteMP3Tags(path string, title string, artist string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	audio := stripExistingTags(data)
	tag := buildID3v23Tag(title, artist)
	return os.WriteFile(path, append(tag, audio...), 0644)
}

// ClearMP3Tags rimuove TUTTI i tag (ID3v2 in testa e ID3v1 in coda) da un file
// MP3, riscrivendo solo il payload audio. Idempotente: su un file già privo di
// tag riscrive gli stessi byte.
func ClearMP3Tags(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path, stripExistingTags(data), 0644)
}

func stripExistingTags(data []byte) []byte {
	if len(data) >= 10 && string(data[:3]) == "ID3" {
		size := syncSafeToInt(data[6:10])
		tagLength := 10 + size
		if data[5]&0x10 != 0 {
			tagLength += 10
		}
		if tagLength <= len(data) {
			data = data[tagLength:]
		}
	}

	if len(data) >= 128 && string(data[len(data)-128:len(data)-125]) == "TAG" {
		data = data[:len(data)-128]
	}

	return data
}

func buildID3v23Tag(title string, artist string) []byte {
	frames := bytes.Buffer{}
	frames.Write(buildTextFrame("TIT2", title))
	frames.Write(buildTextFrame("TPE1", artist))

	body := frames.Bytes()
	header := []byte{'I', 'D', '3', 3, 0, 0, 0, 0, 0, 0}
	copy(header[6:10], intToSyncSafe(len(body)))
	return append(header, body...)
}

func buildTextFrame(id string, value string) []byte {
	payload := bytes.Buffer{}
	payload.WriteByte(0x01)
	payload.Write([]byte{0xff, 0xfe})
	for _, code := range utf16.Encode([]rune(value)) {
		_ = binary.Write(&payload, binary.LittleEndian, code)
	}

	body := payload.Bytes()
	frame := bytes.Buffer{}
	frame.WriteString(id)
	_ = binary.Write(&frame, binary.BigEndian, uint32(len(body)))
	frame.Write([]byte{0, 0})
	frame.Write(body)
	return frame.Bytes()
}

func syncSafeToInt(value []byte) int {
	if len(value) < 4 {
		return 0
	}
	return int(value[0]&0x7f)<<21 |
		int(value[1]&0x7f)<<14 |
		int(value[2]&0x7f)<<7 |
		int(value[3]&0x7f)
}

func intToSyncSafe(value int) []byte {
	return []byte{
		byte((value >> 21) & 0x7f),
		byte((value >> 14) & 0x7f),
		byte((value >> 7) & 0x7f),
		byte(value & 0x7f),
	}
}
