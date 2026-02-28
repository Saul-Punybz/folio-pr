package scraper

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// WebResult holds a single web search result.
type WebResult struct {
	Title   string
	URL     string
	Snippet string
}

// WebSearch performs a DuckDuckGo Lite search and returns parsed results.
// This is used as a fallback when the local article database doesn't have
// relevant results for a user's chat question.
func WebSearch(ctx context.Context, query string, limit int) ([]WebResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	searchURL := "https://lite.duckduckgo.com/lite/?q=" + url.QueryEscape(query)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("websearch: create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("websearch: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("websearch: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, fmt.Errorf("websearch: read body: %w", err)
	}

	return parseDDGLite(string(body), limit), nil
}

// parseDDGLite extracts search results from DuckDuckGo Lite HTML.
// The lite page has a table-based layout with results in specific patterns:
// - Links in <a rel="nofollow" ...> tags with class "result-link"
// - Snippets in <td class="result-snippet"> tags
func parseDDGLite(html string, limit int) []WebResult {
	var results []WebResult

	// DuckDuckGo Lite uses a table layout. Results are in <a rel="nofollow"> links
	// followed by snippet text in subsequent table cells.
	remaining := html
	for len(results) < limit {
		// Find next result link
		linkIdx := strings.Index(remaining, `rel="nofollow"`)
		if linkIdx == -1 {
			break
		}

		// Extract the href from this anchor tag
		// Look backward for href="
		tagStart := strings.LastIndex(remaining[:linkIdx], "<a")
		if tagStart == -1 {
			remaining = remaining[linkIdx+14:]
			continue
		}

			tagEnd := strings.Index(remaining[linkIdx:], ">")
		if tagEnd == -1 {
			remaining = remaining[linkIdx+14:]
			continue
		}
		fullTag := remaining[tagStart : linkIdx+tagEnd+1]

		// Extract href
		href := extractAttr(fullTag, "href")
		if href == "" || strings.Contains(href, "duckduckgo.com") {
			remaining = remaining[linkIdx+tagEnd+1:]
			continue
		}

		// Extract title (text between > and </a>)
		afterTag := remaining[linkIdx+tagEnd+1:]
		closeA := strings.Index(afterTag, "</a>")
		title := ""
		if closeA != -1 {
			title = stripHTML(afterTag[:closeA])
		}

		// Look for snippet in nearby text
		snippet := ""
		snippetIdx := strings.Index(afterTag, "result-snippet")
		if snippetIdx != -1 && snippetIdx < 500 {
			snippetStart := strings.Index(afterTag[snippetIdx:], ">")
			if snippetStart != -1 {
				snippetContent := afterTag[snippetIdx+snippetStart+1:]
				snippetEnd := strings.Index(snippetContent, "</td>")
				if snippetEnd == -1 {
					snippetEnd = strings.Index(snippetContent, "</span>")
				}
				if snippetEnd != -1 {
					snippet = stripHTML(snippetContent[:snippetEnd])
				}
			}
		}

		if title != "" && href != "" {
			results = append(results, WebResult{
				Title:   strings.TrimSpace(title),
				URL:     strings.TrimSpace(href),
				Snippet: strings.TrimSpace(snippet),
			})
		}

		remaining = remaining[linkIdx+tagEnd+1:]
	}

	return results
}

// extractAttr extracts an attribute value from an HTML tag string.
func extractAttr(tag, attr string) string {
	lower := strings.ToLower(tag)
	needle := strings.ToLower(attr) + `="`
	idx := strings.Index(lower, needle)
	if idx == -1 {
		needle = strings.ToLower(attr) + `='`
		idx = strings.Index(lower, needle)
	}
	if idx == -1 {
		return ""
	}
	start := idx + len(needle)
	quote := tag[start-1]
	end := strings.IndexByte(tag[start:], quote)
	if end == -1 {
		return ""
	}
	return tag[start : start+end]
}

// MultiWebSearch runs DDG + Bing News in parallel and merges deduplicated results.
func MultiWebSearch(ctx context.Context, query string, limit int) ([]WebResult, error) {
	type result struct {
		items []WebResult
		err   error
	}

	var wg sync.WaitGroup
	ddgCh := make(chan result, 1)
	bingCh := make(chan result, 1)

	wg.Add(2)
	go func() {
		defer wg.Done()
		items, err := WebSearch(ctx, query, limit)
		ddgCh <- result{items, err}
	}()
	go func() {
		defer wg.Done()
		items, err := BingNewsSearch(ctx, query, limit)
		bingCh <- result{items, err}
	}()
	wg.Wait()

	ddg := <-ddgCh
	bing := <-bingCh

	if ddg.err != nil {
		slog.Warn("multisearch: ddg failed", "err", ddg.err)
	}
	if bing.err != nil {
		slog.Warn("multisearch: bing failed", "err", bing.err)
	}

	// Merge: DDG first, then Bing, deduplicate by URL host+path.
	seen := make(map[string]bool)
	var merged []WebResult

	addResults := func(items []WebResult) {
		for _, item := range items {
			key := normalizeURLKey(item.URL)
			if seen[key] {
				continue
			}
			seen[key] = true
			merged = append(merged, item)
		}
	}

	addResults(ddg.items)
	addResults(bing.items)

	if len(merged) > limit {
		merged = merged[:limit]
	}
	return merged, nil
}

// normalizeURLKey extracts a deduplication key from a URL (host + path, lowercased).
func normalizeURLKey(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return strings.ToLower(rawURL)
	}
	return strings.ToLower(u.Host + u.Path)
}

// stripHTML removes HTML tags and decodes common entities.
func stripHTML(s string) string {
	var out strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			out.WriteRune(r)
		}
	}
	result := out.String()
	result = strings.ReplaceAll(result, "&amp;", "&")
	result = strings.ReplaceAll(result, "&lt;", "<")
	result = strings.ReplaceAll(result, "&gt;", ">")
	result = strings.ReplaceAll(result, "&quot;", `"`)
	result = strings.ReplaceAll(result, "&#x27;", "'")
	result = strings.ReplaceAll(result, "&nbsp;", " ")
	return strings.TrimSpace(result)
}
