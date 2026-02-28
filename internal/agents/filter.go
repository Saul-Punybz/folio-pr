package agents

import (
	"regexp"
	"strings"
)

// isSpamHit returns true if the URL/title/snippet indicate non-PR, NSFW, or irrelevant content.
// This is applied BEFORE inserting into the DB to prevent noise.
// orgKeywords is optional — when provided, Reddit results must mention at least one keyword.
func isSpamHit(url, title, snippet string, orgKeywords ...string) bool {
	lower := strings.ToLower(title + " " + snippet + " " + url)

	// 1. Reddit subreddit homepages (not actual posts)
	if isRedditHomepage(url) {
		return true
	}

	// 2. Generic homepages / aggregator fronts
	if isGenericHomepage(url) {
		return true
	}

	// 3. NSFW / pornographic content
	for _, pat := range nsfwPatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}

	// 4. Non-PR geographic content (unless it also mentions Puerto Rico)
	if !mentionsPR(lower) {
		for _, pat := range nonPRPatterns {
			if strings.Contains(lower, pat) {
				return true
			}
		}
	}

	// 5. Clickbait / low-quality patterns
	for _, pat := range spamPatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}

	// 6. Reddit posts: require they mention at least one org keyword in title/snippet.
	// "Puerto Rico" alone is not enough — many Reddit posts mention PR as a song, meme, etc.
	if strings.Contains(strings.ToLower(url), "reddit.com") && len(orgKeywords) > 0 {
		hasKeyword := false
		for _, kw := range orgKeywords {
			if len(kw) > 1 && strings.Contains(lower, strings.ToLower(kw)) {
				hasKeyword = true
				break
			}
		}
		if !hasKeyword {
			return true
		}
	}

	return false
}

// redditPostRe matches actual Reddit post URLs: /r/sub/comments/id/...
var redditPostRe = regexp.MustCompile(`/r/[^/]+/comments/`)

// isRedditHomepage returns true for Reddit subreddit home pages and non-post URLs.
func isRedditHomepage(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	if !strings.Contains(lower, "reddit.com") {
		return false
	}
	// Actual posts have /r/{sub}/comments/{id}/ — anything else is a homepage or listing
	return !redditPostRe.MatchString(lower)
}

// isGenericHomepage detects generic site front pages with no specific article path.
func isGenericHomepage(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	// Strip protocol
	for _, prefix := range []string{"https://", "http://", "www."} {
		lower = strings.TrimPrefix(lower, prefix)
	}
	// If path is just "/" or empty, it's a homepage
	parts := strings.SplitN(lower, "/", 2)
	if len(parts) < 2 || parts[1] == "" || parts[1] == "/" {
		return true
	}
	return false
}

// mentionsPR checks whether the text mentions Puerto Rico or PR-related terms.
func mentionsPR(lower string) bool {
	for _, term := range prTerms {
		if strings.Contains(lower, term) {
			return true
		}
	}
	return false
}

// prTerms are terms that indicate content is about Puerto Rico.
var prTerms = []string{
	"puerto rico", "boricua", "puertorriqueño", "puertorriquena",
	"san juan", "bayamón", "bayamon", "ponce", "caguas", "mayagüez", "mayaguez",
	"carolina pr", "arecibo", "guaynabo", "isla del encanto",
	".pr/", "gobierno.pr",
}

// nsfwPatterns catch pornographic, adult, and NSFW content.
var nsfwPatterns = []string{
	"onlyfans", "caseros", "porn", "nsfw", "xxx", "nude", "nudes",
	"desnuda", "desnudo", "fotos y videos", "leaks",
	"onlyfan", "fansly", "chaturbate", "manyvids",
	"sexo", "erotico", "erotica", "lenceria",
	"puertoricoleaks", "puertoricanleaks",
	"gonewild", "rule34", "hentai", "milf", "fetish",
}

// nonPRPatterns indicate content about other countries/regions, not Puerto Rico.
var nonPRPatterns = []string{
	"dominicana", "dominicano", "santo domingo", "república dominicana",
	"mexico", "méxico", "colombia", "venezuela", "argentina",
	"españa", "spain", "paraguay", "chile", "perú", "peru",
	"cuba", "panamá", "panama", "ecuador", "bolivia",
	"guatemala", "honduras", "el salvador", "nicaragua", "costa rica",
	"new jersey", "new york city", "florida man",
	"india", "pakistan", "passport india",
}

// spamPatterns catch clickbait, low-quality, or irrelevant content.
var spamPatterns = []string{
	"blind bags", "mystery box", "unboxing haul",
	"little gray alien", "ufo sighting",
	"free v-bucks", "free robux",
	"crypto pump", "bitcoin millionaire",
	"weight loss secret", "diet pill",
	"google noticias", "news.google.com/stories",
}
