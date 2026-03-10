package research

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/scraper"
)

const (
	scrapeTimeout   = 15 * time.Second
	maxScrapePages  = 50
	maxDeepCrawl    = 10
	maxDeepCrawlPer = 3
)

// RunPhase2 scrapes full text from top findings and follows outbound links for deep crawling.
func RunPhase2(ctx context.Context, deps Deps, project *models.ResearchProject) (int, error) {
	slog.Info("research/phase2: starting deep scrape", "topic", project.Topic)

	// Get top unscraped findings
	findings, err := deps.Findings.ListTopUnscraped(ctx, project.ID, maxScrapePages)
	if err != nil {
		return 0, fmt.Errorf("research/phase2: list unscraped: %w", err)
	}

	scraped := 0
	deepCrawled := 0

	for i, f := range findings {
		if ctx.Err() != nil {
			break
		}

		// Scrape the page
		cleanText := scrapePage(ctx, f.URL)
		if cleanText == "" {
			continue
		}

		// AI enrichment: sentiment + entities
		sentiment := "unknown"
		var entitiesJSON json.RawMessage = json.RawMessage("{}")
		var tagsJSON json.RawMessage = json.RawMessage("[]")

		if deps.AI != nil {
			// Classify sentiment
			if s, err := deps.AI.ClassifySentiment(ctx, truncateStr(cleanText, 2000)); err == nil {
				sentiment = s
			}

			// Extract entities
			if ents, err := deps.AI.ExtractEntities(ctx, truncateStr(cleanText, 2000)); err == nil && ents != nil {
				if raw, err := json.Marshal(ents); err == nil {
					entitiesJSON = raw
				}
			}

			// Classify tags
			if tags, err := deps.AI.Classify(ctx, truncateStr(cleanText, 2000)); err == nil && len(tags) > 0 {
				if raw, err := json.Marshal(tags); err == nil {
					tagsJSON = raw
				}
			}
		}

		if err := deps.Findings.UpdateScraped(ctx, f.ID, cleanText, sentiment, tagsJSON, entitiesJSON); err != nil {
			slog.Error("research/phase2: update scraped", "id", f.ID, "err", err)
			continue
		}
		scraped++

		// Deep crawl: for top 10 findings, follow outbound links 1 level deep
		if i < maxDeepCrawl {
			n := deepCrawlPage(ctx, deps, project.ID, f.URL, cleanText)
			deepCrawled += n
		}
	}

	// Update progress
	progress, _ := json.Marshal(map[string]int{
		"phase2_scraped":     scraped,
		"phase2_deep_crawls": deepCrawled,
	})
	_ = deps.Projects.UpdateProgress(ctx, project.ID, progress)

	slog.Info("research/phase2: complete", "topic", project.Topic, "scraped", scraped, "deep_crawled", deepCrawled)
	return scraped, nil
}

// scrapePage fetches a URL and extracts clean text.
func scrapePage(ctx context.Context, rawURL string) string {
	ctx, cancel := context.WithTimeout(ctx, scrapeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return ""
	}

	return stripHTMLTags(string(body))
}

// deepCrawlPage extracts outbound links from page HTML and creates deep_crawl findings.
func deepCrawlPage(ctx context.Context, deps Deps, projectID uuid.UUID, pageURL, pageText string) int {
	// Re-fetch the page to get raw HTML for link extraction
	ctx2, cancel := context.WithTimeout(ctx, scrapeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx2, http.MethodGet, pageURL, nil)
	if err != nil {
		return 0
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return 0
	}

	links := extractLinks(string(body), pageURL)
	hits := 0

	for _, link := range links {
		if hits >= maxDeepCrawlPer || ctx.Err() != nil {
			break
		}

		// Skip non-article links
		if isSkippableURL(link) {
			continue
		}

		f := &models.ResearchFinding{
			ID:         uuid.New(),
			ProjectID:  projectID,
			URL:        link,
			URLHash:    scraper.HashURL(link),
			Title:      "",
			Snippet:    "",
			SourceType: "deep_crawl",
			Sentiment:  "unknown",
		}

		if err := deps.Findings.Create(ctx, f); err != nil {
			continue
		}
		if f.ID == uuid.Nil {
			continue // duplicate
		}

		// Scrape the deep-crawled page
		text := scrapePage(ctx, link)
		if text == "" {
			continue
		}

		// Extract a title from first line
		title := extractTitle(text)

		_ = deps.Findings.UpdateScraped(ctx, f.ID, text, "unknown", json.RawMessage("[]"), json.RawMessage("{}"))
		if title != "" {
			// We can't update title easily so we leave it for phase 3
		}
		hits++
	}

	return hits
}

// extractLinks pulls href values from anchor tags in HTML.
func extractLinks(html, baseURL string) []string {
	var links []string
	seen := make(map[string]bool)
	remaining := html

	for len(links) < 30 {
		idx := strings.Index(remaining, `href="`)
		if idx == -1 {
			break
		}
		remaining = remaining[idx+6:]
		end := strings.Index(remaining, `"`)
		if end == -1 {
			break
		}
		href := remaining[:end]
		remaining = remaining[end+1:]

		// Resolve relative URLs
		if strings.HasPrefix(href, "/") && !strings.HasPrefix(href, "//") {
			// Extract base domain from baseURL
			parts := strings.SplitN(baseURL, "//", 2)
			if len(parts) == 2 {
				domainParts := strings.SplitN(parts[1], "/", 2)
				href = parts[0] + "//" + domainParts[0] + href
			}
		}

		if !strings.HasPrefix(href, "http") {
			continue
		}

		if !seen[href] {
			seen[href] = true
			links = append(links, href)
		}
	}

	return links
}

// isSkippableURL returns true for URLs that aren't article-like.
func isSkippableURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)

	// Skip media files
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".gif", ".svg", ".pdf", ".mp3", ".mp4", ".css", ".js"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}

	// Skip social/auth/generic pages
	skipPatterns := []string{
		"facebook.com", "twitter.com", "instagram.com", "tiktok.com",
		"login", "signup", "register", "mailto:", "javascript:",
		"#", "privacy", "terms", "cookie", "about-us", "contact-us",
	}
	for _, p := range skipPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}

	return false
}

// extractTitle tries to get a title from the first line of clean text.
func extractTitle(text string) string {
	lines := strings.SplitN(text, "\n", 2)
	if len(lines) > 0 {
		title := strings.TrimSpace(lines[0])
		if len(title) > 10 && len(title) < 200 {
			return title
		}
	}
	return ""
}

// stripHTMLTags removes HTML tags and collapses whitespace.
func stripHTMLTags(s string) string {
	var out strings.Builder
	inTag := false
	lastSpace := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			if !lastSpace {
				out.WriteRune(' ')
				lastSpace = true
			}
		case !inTag:
			if r == '\n' || r == '\r' || r == '\t' {
				if !lastSpace {
					out.WriteRune(' ')
					lastSpace = true
				}
			} else {
				out.WriteRune(r)
				lastSpace = r == ' '
			}
		}
	}

	result := strings.TrimSpace(out.String())
	result = strings.ReplaceAll(result, "&amp;", "&")
	result = strings.ReplaceAll(result, "&lt;", "<")
	result = strings.ReplaceAll(result, "&gt;", ">")
	result = strings.ReplaceAll(result, "&quot;", `"`)
	result = strings.ReplaceAll(result, "&#x27;", "'")
	result = strings.ReplaceAll(result, "&nbsp;", " ")
	return result
}
