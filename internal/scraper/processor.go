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

	// Normalize whitespace within lines.
	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(reWhitespace.ReplaceAllString(line, " "))
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
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
