package scraper

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// trackingParams is the set of URL query parameters commonly used for tracking
// that should be stripped during canonicalization.
var trackingParams = map[string]bool{
	"utm_source":   true,
	"utm_medium":   true,
	"utm_campaign": true,
	"utm_term":     true,
	"utm_content":  true,
	"utm_id":       true,
	"fbclid":       true,
	"gclid":        true,
	"gclsrc":       true,
	"dclid":        true,
	"msclkid":      true,
	"twclid":       true,
	"mc_cid":       true,
	"mc_eid":       true,
	"ref":          true,
	"_ga":          true,
	"_gl":          true,
}

// reHTMLTag matches HTML tags.
var reHTMLTag = regexp.MustCompile(`<[^>]*>`)

// reWhitespace matches sequences of whitespace (spaces, tabs, newlines).
var reWhitespace = regexp.MustCompile(`\s+`)

// reBlankLines matches multiple consecutive newlines (after initial cleanup).
var reBlankLines = regexp.MustCompile(`\n{3,}`)

// CleanText strips HTML tags from the input and normalizes whitespace. It
// preserves paragraph boundaries as single newlines.
func CleanText(html string) string {
	if html == "" {
		return ""
	}

	// Replace block-level elements with newlines to preserve paragraph structure.
	blockTags := []string{"</p>", "</div>", "</li>", "</h1>", "</h2>", "</h3>",
		"</h4>", "</h5>", "</h6>", "<br>", "<br/>", "<br />", "</tr>", "</blockquote>"}
	text := html
	for _, tag := range blockTags {
		text = strings.ReplaceAll(strings.ToLower(text), tag, "\n")
		// Also handle the original case.
		text = strings.ReplaceAll(text, tag, "\n")
	}

	// Strip all remaining HTML tags.
	text = reHTMLTag.ReplaceAllString(text, "")

	// Decode common HTML entities.
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", `"`)
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&apos;", "'")
	text = strings.ReplaceAll(text, "&nbsp;", " ")

	// Normalize whitespace within lines and filter out boilerplate.
	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(reWhitespace.ReplaceAllString(line, " "))
		if trimmed == "" {
			continue
		}
		// Skip lines that are clearly boilerplate
		if isBoilerplateLine(trimmed) {
			continue
		}
		cleaned = append(cleaned, trimmed)
	}

	result := strings.Join(cleaned, "\n")

	// Collapse excessive blank lines.
	result = reBlankLines.ReplaceAllString(result, "\n\n")

	return strings.TrimSpace(result)
}

// CanonicalizeURL normalizes a URL by lowercasing the scheme and host, removing
// tracking parameters (utm_*, fbclid, etc.), removing fragments, and sorting
// query parameters.
func CanonicalizeURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL // Return as-is if unparseable.
	}

	// Lowercase scheme and host.
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)

	// Remove fragment.
	parsed.Fragment = ""
	parsed.RawFragment = ""

	// Remove trailing slash from path (unless path is just "/").
	if len(parsed.Path) > 1 {
		parsed.Path = strings.TrimRight(parsed.Path, "/")
	}

	// Filter out tracking query parameters.
	query := parsed.Query()
	for key := range query {
		if trackingParams[strings.ToLower(key)] {
			query.Del(key)
		}
	}

	parsed.RawQuery = query.Encode()

	return parsed.String()
}

// HashContent returns the hex-encoded SHA-256 hash of the given content string.
func HashContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h)
}

// HashURL returns the hex-encoded SHA-256 hash of the canonicalized form of the
// given URL.
func HashURL(rawURL string) string {
	canonical := CanonicalizeURL(rawURL)
	return HashContent(canonical)
}

// CompressGzip compresses the given data using gzip and returns the compressed
// bytes.
func CompressGzip(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("gzip: create writer: %w", err)
	}

	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("gzip: write: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("gzip: close: %w", err)
	}

	return buf.Bytes(), nil
}

// ── Article Content Extraction ─────────────────────────────────────────

// Patterns to remove noise elements and their content (Go regexp has no backreferences).
var reScript = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
var reStyle = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
var reNoscript = regexp.MustCompile(`(?is)<noscript[^>]*>.*?</noscript>`)
var reIframe = regexp.MustCompile(`(?is)<iframe[^>]*>.*?</iframe>`)
var reSVG = regexp.MustCompile(`(?is)<svg[^>]*>.*?</svg>`)
var reSelect = regexp.MustCompile(`(?is)<select[^>]*>.*?</select>`)
var reButton = regexp.MustCompile(`(?is)<button[^>]*>.*?</button>`)

// Patterns to remove boilerplate structural elements.
var reNav = regexp.MustCompile(`(?is)<nav[^>]*>.*?</nav>`)
var reHeader = regexp.MustCompile(`(?is)<header[^>]*>.*?</header>`)
var reFooter = regexp.MustCompile(`(?is)<footer[^>]*>.*?</footer>`)
var reAside = regexp.MustCompile(`(?is)<aside[^>]*>.*?</aside>`)
var reFormTag = regexp.MustCompile(`(?is)<form[^>]*>.*?</form>`)
var reMenu = regexp.MustCompile(`(?is)<menu[^>]*>.*?</menu>`)

// Patterns to remove WordPress/CMS boilerplate divs by class name.
var reWidgetDiv = regexp.MustCompile(`(?is)<div[^>]*class="[^"]*(?:widget|sidebar|breadcrumb|menu-item|wp-block-navigation|site-header|site-footer|cookie-banner|social-share|search-form|skip-link)[^"]*"[^>]*>.*?</div>`)
var reUlMenu = regexp.MustCompile(`(?is)<ul[^>]*class="[^"]*(?:menu|nav-menu|sub-menu|dropdown-menu|wp-block-navigation)[^"]*"[^>]*>.*?</ul>`)

// reComments removes HTML comments.
var reComments = regexp.MustCompile(`(?s)<!--.*?-->`)

// reArticleTag extracts content from <article> tags.
var reArticleTag = regexp.MustCompile(`(?is)<article[^>]*>(.*?)</article>`)

// reMainTag extracts content from <main> tags.
var reMainTag = regexp.MustCompile(`(?is)<main[^>]*>(.*?)</main>`)

// reOGMeta extracts Open Graph meta content.
var reOGImage = regexp.MustCompile(`(?i)<meta\s+(?:[^>]*?)property=["']og:image["']\s+(?:[^>]*?)content=["']([^"']+)["']`)
var reOGImageAlt = regexp.MustCompile(`(?i)<meta\s+(?:[^>]*?)content=["']([^"']+)["']\s+(?:[^>]*?)property=["']og:image["']`)
var reOGDescription = regexp.MustCompile(`(?i)<meta\s+(?:[^>]*?)property=["']og:description["']\s+(?:[^>]*?)content=["']([^"']+)["']`)
var reOGDescAlt = regexp.MustCompile(`(?i)<meta\s+(?:[^>]*?)content=["']([^"']+)["']\s+(?:[^>]*?)property=["']og:description["']`)

// rePublishedTime extracts article:published_time or datePublished.
var rePublishedTime = regexp.MustCompile(`(?i)<meta\s+(?:[^>]*?)(?:property|name)=["'](?:article:published_time|datePublished|pubdate|date)["']\s+(?:[^>]*?)content=["']([^"']+)["']`)
var rePublishedTimeAlt = regexp.MustCompile(`(?i)<meta\s+(?:[^>]*?)content=["']([^"']+)["']\s+(?:[^>]*?)(?:property|name)=["'](?:article:published_time|datePublished|pubdate|date)["']`)

// reTimeTag extracts datetime from <time> elements.
var reTimeTag = regexp.MustCompile(`(?i)<time[^>]+datetime=["']([^"']+)["']`)

// ArticleMeta holds metadata extracted from the page HTML.
type ArticleMeta struct {
	Image       string // og:image URL
	Description string // og:description
	PublishDate string // article:published_time or <time datetime>
}

// ExtractArticle removes boilerplate (nav, header, footer, sidebar, ads, scripts)
// and returns only the article/news content from the page HTML. Also extracts
// metadata (og:image, description, publish date).
func ExtractArticle(html string) (cleanContent string, meta ArticleMeta) {
	if html == "" {
		return "", meta
	}

	// Extract metadata from <head> before stripping
	if m := reOGImage.FindStringSubmatch(html); len(m) > 1 {
		meta.Image = m[1]
	} else if m := reOGImageAlt.FindStringSubmatch(html); len(m) > 1 {
		meta.Image = m[1]
	}

	if m := reOGDescription.FindStringSubmatch(html); len(m) > 1 {
		meta.Description = m[1]
	} else if m := reOGDescAlt.FindStringSubmatch(html); len(m) > 1 {
		meta.Description = m[1]
	}

	if m := rePublishedTime.FindStringSubmatch(html); len(m) > 1 {
		meta.PublishDate = m[1]
	} else if m := rePublishedTimeAlt.FindStringSubmatch(html); len(m) > 1 {
		meta.PublishDate = m[1]
	} else if m := reTimeTag.FindStringSubmatch(html); len(m) > 1 {
		meta.PublishDate = m[1]
	}

	// Step 1: Remove scripts, styles, noscript, iframes, SVGs, selects, buttons
	text := reScript.ReplaceAllString(html, "")
	text = reStyle.ReplaceAllString(text, "")
	text = reNoscript.ReplaceAllString(text, "")
	text = reIframe.ReplaceAllString(text, "")
	text = reSVG.ReplaceAllString(text, "")
	text = reSelect.ReplaceAllString(text, "")
	text = reButton.ReplaceAllString(text, "")

	// Step 2: Remove HTML comments
	text = reComments.ReplaceAllString(text, "")

	// Step 3: Try to find <article> content (best signal for news pages)
	var articleBody string
	if matches := reArticleTag.FindAllStringSubmatch(text, -1); len(matches) > 0 {
		// Use the longest <article> match (some pages have multiple)
		for _, m := range matches {
			if len(m[1]) > len(articleBody) {
				articleBody = m[1]
			}
		}
	}

	// Step 4: Fall back to <main> if no <article>
	if articleBody == "" {
		if m := reMainTag.FindStringSubmatch(text); len(m) > 1 {
			articleBody = m[1]
		}
	}

	// Step 5: If we found article/main content, strip boilerplate from it
	if articleBody != "" {
		articleBody = stripBoilerplate(articleBody)
		cleanContent = CleanText(articleBody)
	} else {
		// No <article> or <main> tag — strip boilerplate from the whole page
		text = stripBoilerplate(text)
		cleanContent = CleanText(text)
	}

	return cleanContent, meta
}

// reJSCode matches common inline JavaScript patterns.
var reJSCode = regexp.MustCompile(`(?i)^(var |let |const |function |ga\(|gtag\(|window\.|document\.|if\s*\(|for\s*\(|\}\s*$|\{\s*$|try\s*\{|catch\s*\()`)

// reCSSProp matches CSS property: value patterns that leak into text.
var reCSSProp = regexp.MustCompile(`(?i)^(margin|padding|border|display|position|font|color|background|width|height|overflow|flex|grid|align|justify|text-align|opacity|z-index|float|clear|visibility|cursor|outline|transform|transition|animation)[\s-]`)

// isBoilerplateLine returns true if the line looks like navigation, JS code,
// CSS, or other non-content boilerplate that should be filtered out.
func isBoilerplateLine(line string) bool {
	lower := strings.ToLower(line)

	// Very short lines are usually navigation items, not content
	if len(line) < 4 {
		return true
	}

	// CSS property declarations that leaked as text
	if reCSSProp.MatchString(line) {
		return true
	}

	// JavaScript code
	if reJSCode.MatchString(line) {
		return true
	}

	// Lines that are just CSS selectors or blocks
	if strings.HasPrefix(line, ".") || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "@") {
		if !strings.HasPrefix(line, "...") && len(line) < 80 {
			return true
		}
	}

	// CSS-like content: property: value;
	if strings.Contains(line, "{") || strings.Contains(line, "}") {
		return true
	}

	// CSS units and functions
	if strings.Contains(lower, "!important") || strings.Contains(lower, "rgba(") || strings.Contains(lower, "hsla(") {
		return true
	}

	// Common nav/boilerplate phrases (exact matches or starts-with)
	boilerplate := []string{
		"skip to content", "skip to main", "ir al contenido",
		"cookie", "accept all", "aceptar todo",
		"search for:", "buscar:", "formulario de busqueda", "formulario de búsqueda",
		"toggle navigation", "toggle menu",
		"copyright", "all rights reserved", "todos los derechos",
		"powered by wordpress", "powered by developer starter",
		"share this", "compartir", "share on facebook", "share on twitter",
		"follow us", "siguenos", "síguenos",
		"back to top", "volver arriba",
		"read more", "leer mas", "leer más",
	}
	for _, bp := range boilerplate {
		if strings.HasPrefix(lower, bp) || lower == bp {
			return true
		}
	}

	return false
}

// stripBoilerplate removes nav, header, footer, aside, form, menu, and CMS widget elements.
func stripBoilerplate(html string) string {
	html = reNav.ReplaceAllString(html, "")
	html = reHeader.ReplaceAllString(html, "")
	html = reFooter.ReplaceAllString(html, "")
	html = reAside.ReplaceAllString(html, "")
	html = reFormTag.ReplaceAllString(html, "")
	html = reMenu.ReplaceAllString(html, "")
	html = reWidgetDiv.ReplaceAllString(html, "")
	html = reUlMenu.ReplaceAllString(html, "")
	return html
}
