package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
)

// SourceSelectors defines the CSS selectors used to extract content from an
// article page.
type SourceSelectors struct {
	TitleSelector string
	BodySelector  string
	DateSelector  string
}

// ScrapedArticle holds the extracted content from a single article page.
type ScrapedArticle struct {
	Title       string
	CleanText   string
	PublishedAt time.Time
	RawHTML     string
}

// Scraper wraps a Colly collector configured with respectful rate limiting.
type Scraper struct {
	userAgent string
}

// NewScraper creates a new Scraper with rate limiting of 1 request/sec per
// domain and at most 2 parallel requests.
func NewScraper() *Scraper {
	return &Scraper{
		userAgent: "Folio/1.0",
	}
}

// newCollector creates a fresh Colly collector with standard settings and rate
// limiting. Each scrape call gets its own collector to avoid state leakage.
func (s *Scraper) newCollector() *colly.Collector {
	c := colly.NewCollector(
		colly.UserAgent(s.userAgent),
		colly.AllowURLRevisit(),
		colly.MaxDepth(1),
	)

	// Rate limit: 1 request per second per domain, 2 parallel requests.
	_ = c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 2,
		Delay:       1 * time.Second,
		RandomDelay: 500 * time.Millisecond,
	})

	// Set respectful headers.
	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "en-US,en;q=0.9,es;q=0.8")
	})

	return c
}

// ScrapeArticle fetches a single article page and extracts its content using the
// provided CSS selectors.
func (s *Scraper) ScrapeArticle(ctx context.Context, articleURL string, selectors SourceSelectors) (*ScrapedArticle, error) {
	c := s.newCollector()

	var (
		result ScrapedArticle
		mu     sync.Mutex
		scrErr error
	)

	// Capture the full HTML of the page.
	c.OnResponse(func(r *colly.Response) {
		mu.Lock()
		result.RawHTML = string(r.Body)
		mu.Unlock()
	})

	// Extract title.
	if selectors.TitleSelector != "" {
		c.OnHTML(selectors.TitleSelector, func(e *colly.HTMLElement) {
			mu.Lock()
			if result.Title == "" {
				result.Title = strings.TrimSpace(e.Text)
			}
			mu.Unlock()
		})
	}

	// Extract body text.
	if selectors.BodySelector != "" {
		c.OnHTML(selectors.BodySelector, func(e *colly.HTMLElement) {
			mu.Lock()
			text := strings.TrimSpace(e.Text)
			if text != "" {
				if result.CleanText != "" {
					result.CleanText += "\n\n"
				}
				result.CleanText += text
			}
			mu.Unlock()
		})
	}

	// Extract date.
	if selectors.DateSelector != "" {
		c.OnHTML(selectors.DateSelector, func(e *colly.HTMLElement) {
			mu.Lock()
			if result.PublishedAt.IsZero() {
				// Try the element text.
				dateStr := strings.TrimSpace(e.Text)
				if dateStr != "" {
					result.PublishedAt = parseDate(dateStr)
				}
				// Also try common attributes: datetime, content.
				if result.PublishedAt.IsZero() {
					if dt := e.Attr("datetime"); dt != "" {
						result.PublishedAt = parseDate(dt)
					}
				}
				if result.PublishedAt.IsZero() {
					if ct := e.Attr("content"); ct != "" {
						result.PublishedAt = parseDate(ct)
					}
				}
			}
			mu.Unlock()
		})
	}

	c.OnError(func(r *colly.Response, err error) {
		mu.Lock()
		scrErr = fmt.Errorf("scraper: fetch %s: %w", articleURL, err)
		mu.Unlock()
	})

	// Respect context cancellation.
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := c.Visit(articleURL); err != nil {
			mu.Lock()
			if scrErr == nil {
				scrErr = fmt.Errorf("scraper: visit %s: %w", articleURL, err)
			}
			mu.Unlock()
		}
		c.Wait()
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-done:
	}

	if scrErr != nil {
		return nil, scrErr
	}

	// Fall back to <title> tag if selector didn't match.
	if result.Title == "" && result.RawHTML != "" {
		result.Title = extractHTMLTitle(result.RawHTML)
	}

	slog.Debug("scraped article", "url", articleURL, "title_len", len(result.Title), "body_len", len(result.CleanText))

	return &result, nil
}

// ScrapeLinks fetches a listing/category page and extracts all matching links.
// Returns a list of absolute URLs.
func (s *Scraper) ScrapeLinks(ctx context.Context, listURL string, linkSelector string) ([]string, error) {
	c := s.newCollector()

	base, err := url.Parse(listURL)
	if err != nil {
		return nil, fmt.Errorf("scraper: parse list URL: %w", err)
	}

	var (
		links  []string
		mu     sync.Mutex
		scrErr error
	)

	c.OnHTML(linkSelector, func(e *colly.HTMLElement) {
		href := strings.TrimSpace(e.Attr("href"))
		if href == "" {
			return
		}

		// Resolve relative URLs to absolute.
		parsed, err := url.Parse(href)
		if err != nil {
			return
		}
		absolute := base.ResolveReference(parsed).String()

		mu.Lock()
		links = append(links, absolute)
		mu.Unlock()
	})

	c.OnError(func(r *colly.Response, err error) {
		mu.Lock()
		scrErr = fmt.Errorf("scraper: fetch links %s: %w", listURL, err)
		mu.Unlock()
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := c.Visit(listURL); err != nil {
			mu.Lock()
			if scrErr == nil {
				scrErr = fmt.Errorf("scraper: visit links %s: %w", listURL, err)
			}
			mu.Unlock()
		}
		c.Wait()
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-done:
	}

	if scrErr != nil {
		return nil, scrErr
	}

	// Deduplicate.
	seen := make(map[string]bool, len(links))
	unique := make([]string, 0, len(links))
	for _, l := range links {
		if !seen[l] {
			seen[l] = true
			unique = append(unique, l)
		}
	}

	slog.Debug("scraped links", "url", listURL, "count", len(unique))

	return unique, nil
}

// ExtractImageURL fetches a page and extracts the og:image or twitter:image meta
// tag content. It returns the image URL or an empty string if none is found.
// The request times out after 10 seconds and never returns an error — it silently
// returns empty on any failure.
func (s *Scraper) ExtractImageURL(ctx context.Context, pageURL string) string {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := s.newCollector()

	var (
		imageURL string
		mu       sync.Mutex
	)

	// Look for og:image.
	c.OnHTML(`meta[property="og:image"]`, func(e *colly.HTMLElement) {
		mu.Lock()
		if imageURL == "" {
			imageURL = strings.TrimSpace(e.Attr("content"))
		}
		mu.Unlock()
	})

	// Fallback: twitter:image.
	c.OnHTML(`meta[name="twitter:image"]`, func(e *colly.HTMLElement) {
		mu.Lock()
		if imageURL == "" {
			imageURL = strings.TrimSpace(e.Attr("content"))
		}
		mu.Unlock()
	})

	c.OnError(func(r *colly.Response, err error) {
		// Silently ignore errors — image extraction is best-effort.
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = c.Visit(pageURL)
		c.Wait()
	}()

	select {
	case <-ctx.Done():
		return ""
	case <-done:
	}

	return imageURL
}

// extractHTMLTitle performs a simple extraction of the <title> tag from raw HTML.
func extractHTMLTitle(html string) string {
	lower := strings.ToLower(html)
	start := strings.Index(lower, "<title")
	if start == -1 {
		return ""
	}
	// Find the closing > of the opening tag.
	tagEnd := strings.Index(html[start:], ">")
	if tagEnd == -1 {
		return ""
	}
	contentStart := start + tagEnd + 1
	end := strings.Index(lower[contentStart:], "</title>")
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(html[contentStart : contentStart+end])
}
