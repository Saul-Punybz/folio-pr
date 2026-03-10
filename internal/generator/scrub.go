package generator

import (
	"strings"
)

// aiPhrasesES contains common AI-generated filler phrases in Spanish.
var aiPhrasesES = []string{
	"es importante destacar",
	"cabe destacar",
	"sin lugar a dudas",
	"en este sentido",
	"es fundamental",
	"vale la pena mencionar",
	"resulta imprescindible",
	"no podemos ignorar",
	"en la actualidad",
	"a lo largo de los años",
	"en el contexto actual",
	"es crucial",
	"dicho esto",
	"en definitiva",
	"sin duda alguna",
	"es necesario señalar",
	"en este orden de ideas",
	"cabe señalar",
	"resulta evidente",
	"se puede afirmar",
	"como resultado de lo anterior",
	"es menester",
	"a la luz de lo anterior",
	"conviene resaltar",
	"no cabe duda",
}

// aiPhrasesEN contains common AI-generated filler phrases in English.
var aiPhrasesEN = []string{
	"it's worth noting",
	"plays a crucial role",
	"in today's world",
	"it is important to note",
	"the landscape of",
	"a testament to",
	"in this regard",
	"it bears mentioning",
	"one cannot overstate",
	"serves as a reminder",
	"at the end of the day",
	"it is worth mentioning",
	"stands as a beacon",
	"in the ever-evolving",
	"navigating the complexities",
}

// allPhrases is the combined set for matching, lowercased.
var allPhrases []string

func init() {
	for _, p := range aiPhrasesES {
		allPhrases = append(allPhrases, strings.ToLower(p))
	}
	for _, p := range aiPhrasesEN {
		allPhrases = append(allPhrases, strings.ToLower(p))
	}
}

// DetectAIPhrases returns all AI-typical phrases found in the content.
func DetectAIPhrases(content string) []string {
	lower := strings.ToLower(content)
	var found []string
	for _, phrase := range allPhrases {
		if strings.Contains(lower, phrase) {
			found = append(found, phrase)
		}
	}
	return found
}

// ScrubAIPhrases removes AI-typical filler phrases from content while
// preserving sentence coherence. It removes the phrase and cleans up
// any resulting awkward punctuation.
func ScrubAIPhrases(content string) string {
	result := content
	for _, phrase := range allPhrases {
		result = removePhraseCI(result, phrase)
	}
	// Clean up double spaces and leading commas after removal.
	result = strings.ReplaceAll(result, "  ", " ")
	result = strings.ReplaceAll(result, " ,", ",")
	result = strings.ReplaceAll(result, ",,", ",")
	result = strings.ReplaceAll(result, ", ,", ",")
	// Clean up sentences that start with ", " after phrase removal.
	result = strings.ReplaceAll(result, ". , ", ". ")
	result = strings.ReplaceAll(result, ".\n, ", ".\n")
	return result
}

// removePhraseCI removes a phrase case-insensitively from text.
func removePhraseCI(text, phrase string) string {
	lower := strings.ToLower(text)
	phraseLen := len(phrase)
	var result strings.Builder
	result.Grow(len(text))
	i := 0
	for i < len(text) {
		idx := strings.Index(lower[i:], phrase)
		if idx == -1 {
			result.WriteString(text[i:])
			break
		}
		result.WriteString(text[i : i+idx])
		i += idx + phraseLen
	}
	return result.String()
}
