package rules

import (
	"regexp"
	"strings"
)

const (
	DefaultStartFolder       = `D:\Musica\Musica Da Convertire`
	DefaultDestinationFolder = `D:\Musica\Musica Convertita`
)

var OccurrenciesToRemove = []string{
	"(Official Music Video)", "(Official Visual)", "(Lyrics)", "(Visual)", "(freestyle)", "(Video Animation)",
	"(Original Mix)", "(Lyrics/Lyric Video)", "(Visual)", "(Lyrics Video)", "(Official Video)", "(Visual Video)",
	"(Lyric Video), (Official Audio)",
}

var OccurrenciesToReplaceWithFt = []string{
	"featuring", "feat.", "Feat.", "ft.", "Ft.", "feat", "Feat",
}

var SupportedExtensions = []string{
	"ogg", "mp3", "mp4", "m4a", "aac", "flac",
}

var squareParenthesesPattern = regexp.MustCompile(`\[.*?\]`)
var spacesPattern = regexp.MustCompile(` +`)

func IsSupportedExtension(ext string) bool {
	for _, supported := range SupportedExtensions {
		if ext == supported {
			return true
		}
	}
	return false
}

func NormalizeFileBase(title string) string {
	title = removeOccurrencies(title)
	title = replaceFt(title)
	title = replaceFtParenthesis(title)
	title = replaceVIP(title)
	title = removeBetweenSquareParenthesis(title)
	title = swapReCrankWithRemix(title)
	title = swapThaSupreme(title)
	title = swapProd(title)
	title = changeCapitalX(title)
	title = removeTripleAndDoubleSpacesAndTrim(title)
	title = removeDashAtStart(title)
	return strings.TrimSpace(title)
}

func removeOccurrencies(title string) string {
	for _, occurrence := range OccurrenciesToRemove {
		title = strings.ReplaceAll(title, occurrence, "")
	}
	return title
}

func replaceFt(title string) string {
	for _, occurrence := range OccurrenciesToReplaceWithFt {
		title = strings.ReplaceAll(title, occurrence, "ft")
	}
	return title
}

func replaceFtParenthesis(title string) string {
	if strings.Index(title, "(ft") > 0 {
		parts := strings.Split(title, "(ft")
		title = parts[0]
		title += "ft" + strings.Replace(parts[1], ")", "", 1)
	}
	return title
}

func replaceVIP(title string) string {
	return strings.ReplaceAll(title, "(VIP)", "VIP")
}

func removeBetweenSquareParenthesis(title string) string {
	return squareParenthesesPattern.ReplaceAllString(title, "")
}

func swapReCrankWithRemix(title string) string {
	return strings.ReplaceAll(title, " Re-Crank", " Remix")
}

func swapThaSupreme(title string) string {
	return strings.ReplaceAll(title, "tha Supreme", "thasup")
}

func swapProd(title string) string {
	return strings.ReplaceAll(title, "Prod.", "prod.")
}

func changeCapitalX(title string) string {
	return strings.ReplaceAll(title, " X ", " x ")
}

func removeTripleAndDoubleSpacesAndTrim(title string) string {
	return spacesPattern.ReplaceAllString(title, " ")
}

func removeDashAtStart(title string) string {
	if strings.HasPrefix(title, "-") {
		return title[1:]
	}
	return title
}
