package rules

import (
	"regexp"
	"strings"
)

const (
	DefaultStartFolder       = `D:\Musica\Musica Da Convertire`
	DefaultDestinationFolder = `D:\Musica\Musica Convertita`
)

// Replacement è una sostituzione testuale generica From -> To,
// applicata in ordine dalla pipeline di normalizzazione.
type Replacement struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Config raccoglie tutte le regole configurabili in-sessione.
// I passi strutturali (rimozione di [...], collapse spazi, trim,
// dash iniziale, "(ft" -> "ft") restano fissi in NormalizeFileBase.
type Config struct {
	StartFolder                 string        `json:"startFolder"`
	SupportedExtensions         []string      `json:"supportedExtensions"`
	OccurrenciesToRemove        []string      `json:"occurrenciesToRemove"`
	OccurrenciesToReplaceWithFt []string      `json:"occurrenciesToReplaceWithFt"`
	Replacements                []Replacement `json:"replacements"`
}

// FactoryConfig è il seed di fabbrica usato SOLO al primo avvio per inizializzare
// i file persistiti (config.json e defaults.json). Dopo il primo avvio le regole
// vivono su disco e sono modificabili dall'utente.
func FactoryConfig() Config {
	return Config{
		StartFolder: "", // al primo avvio nessuna cartella selezionata
		SupportedExtensions: []string{
			"mp3", "flac", "m4a", "aac", "mp4", "ogg", "opus", "wav", "wma", "aiff",
		},
		OccurrenciesToRemove: []string{
			"(Official Music Video)",
			"(Official Video)",
			"(Official Audio)",
			"(Official Lyric Video)",
			"(Official Lyrics Video)",
			"(Official Visualizer)",
			"(Official Visual)",
			"(Visualizer)",
			"(Visual Video)",
			"(Visual)",
			"(Lyric Video)",
			"(Lyrics Video)",
			"(Lyrics/Lyric Video)",
			"(Lyrics)",
			"(Lyric)",
			"(Audio)",
			"(Music Video)",
			"(Video)",
			"(Video Animation)",
			"(Original Mix)",
			"(Explicit)",
			"(Clean)",
			"(HD)",
			"(HQ)",
			"(4K)",
			"(Full HD)",
			"(freestyle)",
			"(Free Download)",
			"(Free DL)",
			"(Color Coded Lyrics)",
			"(Colour Coded Lyrics)",
			"(Out Now)",
			"(OUT NOW)",
			"(Premiere)",
			"(NCS Release)",
			"(No Copyright Music)",
			"(Bass Boosted)",
		},
		OccurrenciesToReplaceWithFt: []string{
			"featuring", "Featuring", "FEATURING",
			"feat.", "Feat.", "FEAT.",
			"ft.", "Ft.", "FT.",
			"feat", "Feat", "FEAT",
		},
		Replacements: []Replacement{
			{From: "–", To: "-"},   // en dash -> trattino (es. "Artista – Titolo" di YouTube)
			{From: "—", To: "-"},   // em dash -> trattino
			{From: "’", To: "'"},   // apostrofo tipografico -> apostrofo semplice
			{From: "‘", To: "'"},   // apostrofo tipografico (sinistro) -> apostrofo semplice
			{From: " w/ ", To: " ft "},
			{From: "_", To: " "},
			{From: "(VIP)", To: "VIP"},
			{From: " Re-Crank", To: " Remix"},
			{From: "tha Supreme", To: "thasup"},
			{From: "Prod.", To: "prod."},
			{From: " X ", To: " x "},
		},
	}
}

var squareParenthesesPattern = regexp.MustCompile(`\[.*?\]`)
var spacesPattern = regexp.MustCompile(` +`)

func (c Config) IsSupportedExtension(ext string) bool {
	for _, supported := range c.SupportedExtensions {
		if ext == supported {
			return true
		}
	}
	return false
}

// NormalizeFileBase applica la pipeline di normalizzazione nello stesso
// ordine del progetto Java originale.
func (c Config) NormalizeFileBase(title string) string {
	title = c.removeOccurrencies(title)
	title = c.replaceFt(title)
	title = replaceFtParenthesis(title)
	title = c.applyReplacements(title)
	title = removeBetweenSquareParenthesis(title)
	title = removeTripleAndDoubleSpacesAndTrim(title)
	title = removeDashAtStart(title)
	return strings.TrimSpace(title)
}

func (c Config) removeOccurrencies(title string) string {
	for _, occurrence := range c.OccurrenciesToRemove {
		title = strings.ReplaceAll(title, occurrence, "")
	}
	return title
}

func (c Config) replaceFt(title string) string {
	for _, occurrence := range c.OccurrenciesToReplaceWithFt {
		title = strings.ReplaceAll(title, occurrence, "ft")
	}
	return title
}

func (c Config) applyReplacements(title string) string {
	for _, r := range c.Replacements {
		title = strings.ReplaceAll(title, r.From, r.To)
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

func removeBetweenSquareParenthesis(title string) string {
	return squareParenthesesPattern.ReplaceAllString(title, "")
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
