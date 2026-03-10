package generator

import (
	"math"
	"strings"
	"unicode"
)

// SEOScore holds the structured breakdown of an article's SEO quality.
type SEOScore struct {
	Total           int              `json:"total"`
	KeywordDensity  ComponentScore   `json:"keyword_density"`
	TitleQuality    ComponentScore   `json:"title_quality"`
	MetaDescription ComponentScore   `json:"meta_description"`
	Readability     ComponentScore   `json:"readability"`
	Structure       ComponentScore   `json:"structure"`
	AICleanliness   ComponentScore   `json:"ai_cleanliness"`
	Warnings        []string         `json:"warnings,omitempty"`
}

// ComponentScore holds a single SEO component's score.
type ComponentScore struct {
	Score   int    `json:"score"`
	Max     int    `json:"max"`
	Details string `json:"details"`
}

// ScoreArticle evaluates an article's SEO quality on a 100-point scale.
// All scoring is pure Go — no AI calls needed.
func ScoreArticle(content, title, metaDesc, primaryKeyword string) SEOScore {
	score := SEOScore{}
	var warnings []string

	// 1. Keyword Density (20 points) — ideal range 1-2%
	score.KeywordDensity = scoreKeywordDensity(content, primaryKeyword, 20)

	// 2. Title Quality (15 points) — 50-60 chars + contains keyword
	score.TitleQuality = scoreTitleQuality(title, primaryKeyword, 15)

	// 3. Meta Description (15 points) — 150-160 chars + contains keyword
	score.MetaDescription = scoreMetaDescription(metaDesc, primaryKeyword, 15)

	// 4. Readability (15 points) — sentence/paragraph length
	score.Readability = scoreReadability(content, 15)

	// 5. Structure (20 points) — H2 count, FAQ, conclusion, word count
	score.Structure = scoreStructure(content, 20)

	// 6. AI Cleanliness (15 points) — penalize per AI phrase
	score.AICleanliness = scoreAICleanliness(content, 15)

	score.Total = score.KeywordDensity.Score + score.TitleQuality.Score +
		score.MetaDescription.Score + score.Readability.Score +
		score.Structure.Score + score.AICleanliness.Score

	// Collect warnings
	if score.KeywordDensity.Score < 10 {
		warnings = append(warnings, "Densidad de keyword fuera del rango ideal (1-2%)")
	}
	if score.TitleQuality.Score < 8 {
		warnings = append(warnings, "El titulo puede mejorarse (50-60 caracteres, incluir keyword)")
	}
	if score.MetaDescription.Score < 8 {
		warnings = append(warnings, "La meta description puede mejorarse (150-160 caracteres)")
	}
	if score.AICleanliness.Score < 10 {
		warnings = append(warnings, "Se detectaron frases tipicas de IA en el contenido")
	}
	if score.Structure.Score < 12 {
		warnings = append(warnings, "La estructura puede mejorarse (mas secciones H2, FAQ, conclusion)")
	}

	score.Warnings = warnings
	return score
}

func scoreKeywordDensity(content, keyword string, maxPoints int) ComponentScore {
	if keyword == "" {
		return ComponentScore{Score: maxPoints / 2, Max: maxPoints, Details: "sin keyword primario"}
	}
	words := countWordsInText(content)
	if words == 0 {
		return ComponentScore{Score: 0, Max: maxPoints, Details: "sin contenido"}
	}

	lower := strings.ToLower(content)
	kwLower := strings.ToLower(keyword)
	count := strings.Count(lower, kwLower)
	density := (float64(count) / float64(words)) * 100

	var score int
	var details string
	switch {
	case density >= 1.0 && density <= 2.0:
		score = maxPoints
		details = formatFloat(density) + "% (ideal)"
	case density >= 0.5 && density < 1.0:
		score = int(float64(maxPoints) * 0.7)
		details = formatFloat(density) + "% (bajo)"
	case density > 2.0 && density <= 3.0:
		score = int(float64(maxPoints) * 0.7)
		details = formatFloat(density) + "% (alto)"
	case density > 3.0:
		score = int(float64(maxPoints) * 0.3)
		details = formatFloat(density) + "% (excesivo)"
	default:
		score = int(float64(maxPoints) * 0.3)
		details = formatFloat(density) + "% (muy bajo)"
	}

	return ComponentScore{Score: score, Max: maxPoints, Details: details}
}

func scoreTitleQuality(title, keyword string, maxPoints int) ComponentScore {
	titleLen := len([]rune(title))
	score := 0
	var parts []string

	// Length scoring (up to 9 points)
	switch {
	case titleLen >= 50 && titleLen <= 60:
		score += 9
		parts = append(parts, "largo ideal")
	case titleLen >= 40 && titleLen <= 70:
		score += 6
		parts = append(parts, "largo aceptable")
	case titleLen > 0:
		score += 3
		parts = append(parts, "largo suboptimo")
	default:
		parts = append(parts, "sin titulo")
	}

	// Keyword presence (up to 6 points)
	if keyword != "" && strings.Contains(strings.ToLower(title), strings.ToLower(keyword)) {
		score += 6
		parts = append(parts, "contiene keyword")
	} else if keyword != "" {
		parts = append(parts, "falta keyword")
	}

	if score > maxPoints {
		score = maxPoints
	}

	return ComponentScore{Score: score, Max: maxPoints, Details: strings.Join(parts, ", ")}
}

func scoreMetaDescription(metaDesc, keyword string, maxPoints int) ComponentScore {
	descLen := len([]rune(metaDesc))
	score := 0
	var parts []string

	// Length scoring (up to 9 points)
	switch {
	case descLen >= 150 && descLen <= 160:
		score += 9
		parts = append(parts, "largo ideal")
	case descLen >= 120 && descLen <= 180:
		score += 6
		parts = append(parts, "largo aceptable")
	case descLen > 0:
		score += 3
		parts = append(parts, "largo suboptimo")
	default:
		parts = append(parts, "sin meta description")
	}

	// Keyword presence (up to 6 points)
	if keyword != "" && strings.Contains(strings.ToLower(metaDesc), strings.ToLower(keyword)) {
		score += 6
		parts = append(parts, "contiene keyword")
	} else if keyword != "" {
		parts = append(parts, "falta keyword")
	}

	if score > maxPoints {
		score = maxPoints
	}

	return ComponentScore{Score: score, Max: maxPoints, Details: strings.Join(parts, ", ")}
}

func scoreReadability(content string, maxPoints int) ComponentScore {
	sentences := countSentences(content)
	words := countWordsInText(content)
	paragraphs := countParagraphs(content)

	if sentences == 0 || words == 0 {
		return ComponentScore{Score: 0, Max: maxPoints, Details: "sin contenido"}
	}

	avgWordsPerSentence := float64(words) / float64(sentences)
	avgSentencesPerParagraph := float64(sentences) / math.Max(float64(paragraphs), 1)

	score := 0
	var parts []string

	// Sentence length (up to 8 points) — ideal 15-25 words
	switch {
	case avgWordsPerSentence >= 15 && avgWordsPerSentence <= 25:
		score += 8
		parts = append(parts, formatFloat(avgWordsPerSentence)+" pal/orac (ideal)")
	case avgWordsPerSentence >= 10 && avgWordsPerSentence <= 30:
		score += 5
		parts = append(parts, formatFloat(avgWordsPerSentence)+" pal/orac")
	default:
		score += 2
		parts = append(parts, formatFloat(avgWordsPerSentence)+" pal/orac (fuera rango)")
	}

	// Paragraph length (up to 7 points) — ideal 2-5 sentences
	switch {
	case avgSentencesPerParagraph >= 2 && avgSentencesPerParagraph <= 5:
		score += 7
		parts = append(parts, formatFloat(avgSentencesPerParagraph)+" orac/parr (ideal)")
	case avgSentencesPerParagraph >= 1 && avgSentencesPerParagraph <= 7:
		score += 4
		parts = append(parts, formatFloat(avgSentencesPerParagraph)+" orac/parr")
	default:
		score += 2
		parts = append(parts, formatFloat(avgSentencesPerParagraph)+" orac/parr (fuera rango)")
	}

	if score > maxPoints {
		score = maxPoints
	}

	return ComponentScore{Score: score, Max: maxPoints, Details: strings.Join(parts, ", ")}
}

func scoreStructure(content string, maxPoints int) ComponentScore {
	h2Count := strings.Count(content, "\n## ") + strings.Count(content, "\n##\t")
	// Check if first line is an H2
	if strings.HasPrefix(content, "## ") {
		h2Count++
	}

	wordCount := countWordsInText(content)
	lower := strings.ToLower(content)
	hasFAQ := strings.Contains(lower, "preguntas frecuentes") || strings.Contains(lower, "faq")
	hasConclusion := strings.Contains(lower, "conclusion") || strings.Contains(lower, "resumen") || strings.Contains(lower, "consideraciones finales")

	score := 0
	var parts []string

	// H2 count (up to 8 points) — ideal 3-6
	switch {
	case h2Count >= 3 && h2Count <= 6:
		score += 8
		parts = append(parts, formatInt(h2Count)+" secciones H2 (ideal)")
	case h2Count >= 2:
		score += 5
		parts = append(parts, formatInt(h2Count)+" secciones H2")
	case h2Count >= 1:
		score += 3
		parts = append(parts, formatInt(h2Count)+" seccion H2")
	default:
		parts = append(parts, "sin secciones H2")
	}

	// Word count (up to 6 points) — target 1200+
	switch {
	case wordCount >= 1200:
		score += 6
		parts = append(parts, formatInt(wordCount)+" palabras (ideal)")
	case wordCount >= 800:
		score += 4
		parts = append(parts, formatInt(wordCount)+" palabras")
	case wordCount >= 400:
		score += 2
		parts = append(parts, formatInt(wordCount)+" palabras (corto)")
	default:
		parts = append(parts, formatInt(wordCount)+" palabras (muy corto)")
	}

	// FAQ section (up to 3 points)
	if hasFAQ {
		score += 3
		parts = append(parts, "tiene FAQ")
	}

	// Conclusion (up to 3 points)
	if hasConclusion {
		score += 3
		parts = append(parts, "tiene conclusion")
	}

	if score > maxPoints {
		score = maxPoints
	}

	return ComponentScore{Score: score, Max: maxPoints, Details: strings.Join(parts, ", ")}
}

func scoreAICleanliness(content string, maxPoints int) ComponentScore {
	found := DetectAIPhrases(content)
	count := len(found)

	score := maxPoints - (count * 3)
	if score < 0 {
		score = 0
	}

	var details string
	if count == 0 {
		details = "sin frases de IA detectadas"
	} else {
		details = formatInt(count) + " frases de IA encontradas"
	}

	return ComponentScore{Score: score, Max: maxPoints, Details: details}
}

// Helper functions

func countWordsInText(text string) int {
	count := 0
	inWord := false
	for _, r := range text {
		if unicode.IsSpace(r) {
			inWord = false
		} else if !inWord {
			inWord = true
			count++
		}
	}
	return count
}

func countSentences(text string) int {
	count := 0
	for _, r := range text {
		if r == '.' || r == '!' || r == '?' {
			count++
		}
	}
	if count == 0 && len(text) > 0 {
		count = 1
	}
	return count
}

func countParagraphs(text string) int {
	paragraphs := 0
	lines := strings.Split(text, "\n")
	inParagraph := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			inParagraph = false
		} else if !inParagraph {
			inParagraph = true
			paragraphs++
		}
	}
	if paragraphs == 0 && len(text) > 0 {
		paragraphs = 1
	}
	return paragraphs
}

func formatFloat(f float64) string {
	s := strings.TrimRight(strings.TrimRight(
		strings.Replace(
			strings.Replace(
				strings.Replace(
					formatFloatRaw(f), ".", ".", 1),
				",", "", -1),
			" ", "", -1),
		"0"), ".")
	return s
}

func formatFloatRaw(f float64) string {
	return strings.TrimRight(strings.TrimRight(
		func() string {
			i := int(f * 10)
			return formatInt(i/10) + "." + formatInt(i%10)
		}(), "0"), ".")
}

func formatInt(n int) string {
	if n < 0 {
		return "-" + formatInt(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return formatInt(n/10) + string(rune('0'+n%10))
}
