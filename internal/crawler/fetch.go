package crawler

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"

	"github.com/Saul-Punybz/folio/internal/scraper"
)

// DiscoveredLink represents a link found on a crawled page.
type DiscoveredLink struct {
	URL        string
	AnchorText string
	IsExternal bool
}

// FetchResult holds the data extracted from a single fetched page.
type FetchResult struct {
	URL           string
	StatusCode    int
	Title         string
	CleanText     string
	RawHTML       string
	ContentHash   string
	ContentType   string
	ContentLength int
	Links         []DiscoveredLink
}

// FetchPage fetches a single page and extracts content + links. Uses a fresh
// Colly collector with respectful rate limiting.
func FetchPage(ctx context.Context, pageURL string, allowedDomains map[string]bool) (*FetchResult, error) {
	c := colly.NewCollector(
		colly.UserAgent("FolioBot/1.0 (+https://folio.pr)"),
		colly.AllowURLRevisit(),
		colly.MaxDepth(0),
	)

	_ = c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 2,
		Delay:       1 * time.Second,
		RandomDelay: 500 * time.Millisecond,
	})

	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "en-US,en;q=0.9,es;q=0.8")
	})

	parsedBase, err := url.Parse(pageURL)
	if err != nil {
		return nil, fmt.Errorf("fetch: parse URL: %w", err)
	}
	baseDomain := strings.ToLower(parsedBase.Hostname())

	var (
		result FetchResult
		mu     sync.Mutex
		scrErr error
	)
	result.URL = pageURL

	c.OnResponse(func(r *colly.Response) {
		mu.Lock()
		result.StatusCode = r.StatusCode
		result.RawHTML = string(r.Body)
		result.ContentType = r.Headers.Get("Content-Type")
		result.ContentLength = len(r.Body)
		mu.Unlock()
	})

	// Extract title from <title> tag
	c.OnHTML("title", func(e *colly.HTMLElement) {
		mu.Lock()
		if result.Title == "" {
			result.Title = strings.TrimSpace(e.Text)
		}
		mu.Unlock()
	})

	// Also try og:title
	c.OnHTML(`meta[property="og:title"]`, func(e *colly.HTMLElement) {
		mu.Lock()
		if result.Title == "" {
			result.Title = strings.TrimSpace(e.Attr("content"))
		}
		mu.Unlock()
	})

	// Extract all links
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		href := strings.TrimSpace(e.Attr("href"))
		if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") || strings.HasPrefix(href, "mailto:") {
			return
		}

		parsed, parseErr := url.Parse(href)
		if parseErr != nil {
			return
		}
		abs := parsedBase.ResolveReference(parsed)
		absStr := abs.String()

		linkDomain := strings.ToLower(abs.Hostname())
		isExternal := linkDomain != baseDomain && !allowedDomains[linkDomain]

		anchor := strings.TrimSpace(e.Text)
		if len(anchor) > 200 {
			anchor = anchor[:200]
		}

		mu.Lock()
		result.Links = append(result.Links, DiscoveredLink{
			URL:        absStr,
			AnchorText: anchor,
			IsExternal: isExternal,
		})
		mu.Unlock()
	})

	c.OnError(func(r *colly.Response, err error) {
		mu.Lock()
		if r != nil {
			result.StatusCode = r.StatusCode
		}
		scrErr = fmt.Errorf("fetch %s: %w", pageURL, err)
		mu.Unlock()
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		if visitErr := c.Visit(pageURL); visitErr != nil {
			mu.Lock()
			if scrErr == nil {
				scrErr = fmt.Errorf("fetch visit %s: %w", pageURL, visitErr)
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
		return &result, scrErr
	}

	// Extract clean text from HTML
	if result.RawHTML != "" {
		result.CleanText = scraper.CleanText(result.RawHTML)
		result.ContentHash = scraper.HashContent(result.CleanText)
	}

	return &result, nil
}
