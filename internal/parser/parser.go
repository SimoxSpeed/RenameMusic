package parser

import (
	"strings"
)

const (
	UnknownArtist = "Artista Sconosciuto"
	UnknownTitle  = "Titolo Sconosciuto"
)

func Extension(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx < 0 || idx == len(name)-1 {
		return ""
	}
	return name[idx+1:]
}

func RemoveExtension(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx < 0 {
		return name
	}
	return name[:idx]
}

// TagArtist deduce l'artista (o gli artisti) dal nome file. `exceptions` elenca i
// nomi d'arte con " & "/" x " che non vanno spezzati (vedi ReplaceWithComma).
func TagArtist(fileName string, exceptions []string) string {
	title := RemoveExtension(fileName)
	artists := ""

	if IsRemixed(title) {
		artists = substring(title, strings.LastIndex(title, " (")+2, strings.LastIndex(title, " "))
		artists = ReplaceWithComma(artists, exceptions) + ", "
		firstRemixer := substring(title, 0, strings.LastIndex(title, " ("))
		if IsRemixed(firstRemixer) {
			firstRemixer = substring(firstRemixer, strings.LastIndex(firstRemixer, " (")+2, strings.LastIndex(firstRemixer, " "))
			artists += ReplaceWithComma(firstRemixer, exceptions) + ", "
		}
	}

	if strings.Contains(title, " - ") {
		artists += ReplaceWithComma(javaSplit(title, " - ")[0], exceptions)
		if strings.Contains(title, " ft ") {
			artists += ", "
			if IsRemixed(title) {
				artists += ReplaceWithComma(substring(title, strings.Index(title, " ft ")+4, strings.LastIndex(title, " (")), exceptions)
			} else if vipAfterFt(title) {
				artists += ReplaceWithComma(substring(title, strings.Index(title, " ft ")+4, strings.LastIndex(title, " VIP")), exceptions)
			} else {
				artists += ReplaceWithComma(substring(title, strings.Index(title, " ft ")+4, len(title)), exceptions)
			}
		}
	} else {
		artists += UnknownArtist
	}

	return removeDuplicates(artists)
}

// TagTitle deduce il titolo dal nome file. `exceptions` come in TagArtist.
func TagTitle(fileName string, exceptions []string) string {
	title := RemoveExtension(fileName)
	if strings.Contains(title, " - ") {
		parts := javaSplit(title, " - ")
		if len(parts) < 2 {
			return UnknownTitle
		}
		tagTitle := parts[1]
		if strings.Contains(tagTitle, " ft ") {
			if IsRemixed(tagTitle) {
				ret := ReplaceWithComma(substring(tagTitle, strings.Index(tagTitle, " ft ")+4, strings.LastIndex(tagTitle, " (")), exceptions)
				return substring(tagTitle, 0, strings.Index(tagTitle, " ft ")) + " ft " + ret + substring(tagTitle, strings.LastIndex(tagTitle, " ("), len(tagTitle))
			} else if vipAfterFt(tagTitle) {
				ret := ReplaceWithComma(substring(tagTitle, strings.Index(tagTitle, " ft ")+4, strings.LastIndex(tagTitle, " VIP")), exceptions)
				return substring(tagTitle, 0, strings.Index(tagTitle, " ft ")) + " ft " + ret + " VIP"
			}
			return tagTitle
		}
		return tagTitle
	}
	return UnknownTitle
}

func IsVIP(title string) bool {
	return strings.Contains(title, " VIP ")
}

// vipAfterFt indica se un marcatore " VIP" compare DOPO " ft ": solo in quel caso
// "VIP" va trattato come suffisso finale (dopo gli artisti featuring). Se "VIP"
// fa parte del titolo prima di "ft" (es. "Mock VIP ft Jabra") non si applica.
func vipAfterFt(s string) bool {
	ft := strings.Index(s, " ft ")
	vip := strings.LastIndex(s, " VIP")
	return ft >= 0 && vip > ft
}

func IsRemixed(title string) bool {
	hasRemixMarker := strings.Contains(title, "Remix)") ||
		strings.Contains(title, "Flip)") ||
		strings.Contains(title, "Bootleg)") ||
		strings.Contains(title, "Edit)") ||
		strings.Contains(title, "VIP)") ||
		strings.Contains(title, "Mashup)") ||
		strings.Contains(title, "Cover)") ||
		strings.Contains(title, "Treat)")

	hasPlainMarker := strings.Contains(title, "(Remix)") ||
		strings.Contains(title, "(Flip)") ||
		strings.Contains(title, "(Bootleg)") ||
		strings.Contains(title, "(Edit)") ||
		strings.Contains(title, "(VIP)") ||
		strings.Contains(title, "(Mashup)") ||
		strings.Contains(title, "(Cover)") ||
		strings.Contains(title, "(Treat)")

	return hasRemixMarker && !hasPlainMarker
}

// ReplaceWithComma converte i separatori multi-artista (" & " e " x ") in ", ",
// preservando però i nomi d'arte elencati in `exceptions` (es. "Jkyl & Hyde"),
// che non vanno spezzati. Per ogni eccezione si calcola la sua forma "spezzata"
// (applicando la stessa conversione) e la si ripristina alla forma originale.
func ReplaceWithComma(value string, exceptions []string) string {
	value = strings.ReplaceAll(value, " & ", ", ")
	value = strings.ReplaceAll(value, " x ", ", ")
	for _, ex := range exceptions {
		broken := strings.ReplaceAll(ex, " & ", ", ")
		broken = strings.ReplaceAll(broken, " x ", ", ")
		if broken == ex {
			continue // nessun separatore da preservare
		}
		value = strings.ReplaceAll(value, broken, ex)
	}
	return value
}

func removeDuplicates(artists string) string {
	parts := javaSplit(artists, ", ")
	seen := make(map[string]bool, len(parts))
	ordered := make([]string, 0, len(parts))
	for _, part := range parts {
		if !seen[part] {
			seen[part] = true
			ordered = append(ordered, part)
		}
	}
	return strings.Join(ordered, ", ")
}

func javaSplit(value string, sep string) []string {
	parts := strings.Split(value, sep)
	for len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	if len(parts) == 0 {
		return []string{""}
	}
	return parts
}

func substring(value string, start int, end int) string {
	if start < 0 || end < start || start > len(value) {
		return ""
	}
	if end > len(value) {
		end = len(value)
	}
	return value[start:end]
}
