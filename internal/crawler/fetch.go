package crawler

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"

	"github.com/Saul-Punybz/folio/internal/pdftext"
	"github.com/Saul-Punybz/folio/internal/scraper"
)

// UserAgent de FolioBot. Sin URL: folio.pr no es del dueño.
const UserAgent = "FolioBot/1.0"

// MaxDocumentBytes: tope de descarga (igual que pdftext).
const MaxDocumentBytes = 40 << 20

// fetchTimeout: tope por petición (un PDF de 40 MB en una conexión lenta).
const fetchTimeout = 90 * time.Second

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
	Body          []byte // cuerpo tal cual (para PDF)
	IsPDF         bool   // por Content-Type o firma %PDF-
	TooLarge      bool   // pasó de MaxDocumentBytes: se descarta
	FinalURL      string // tras redirecciones
	ETag          string // validadores del servidor (para no volver a bajar un PDF)
	LastModified  string
	Links         []DiscoveredLink
	ImageURL      string
	Description   string
	PublishDate   string
}

// FetchPage fetches a single page (HTML o PDF) and extracts content + links.
// Uses a fresh Colly collector; la pausa entre peticiones al mismo host y
// robots.txt los maneja RunCrawl antes de llamar aquí.
func FetchPage(ctx context.Context, pageURL string, allowedDomains map[string]bool) (*FetchResult, error) {
	c := colly.NewCollector(
		colly.UserAgent(UserAgent),
		colly.AllowURLRevisit(),
		colly.MaxDepth(0),
		// Un byte más que el tope: si llega a eso, el archivo era más grande
		// (Colly corta en silencio).
		colly.MaxBodySize(MaxDocumentBytes+1),
	)
	c.SetRequestTimeout(fetchTimeout)

	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,application/pdf;q=0.9,*/*;q=0.8")
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
		result.ContentType = r.Headers.Get("Content-Type")
		result.ContentLength = len(r.Body)
		result.FinalURL = r.Request.URL.String()
		if len(r.Body) > MaxDocumentBytes {
			result.TooLarge = true
		} else if pdftext.IsPDF(result.ContentType, r.Body) {
			result.IsPDF = true
			result.Body = r.Body
		} else {
			result.RawHTML = string(r.Body)
		}
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
		base := parsedBase
		if e.Request != nil && e.Request.URL != nil {
			base = e.Request.URL // tras redirección, los relativos van contra la URL final
		}
		abs := base.ResolveReference(parsed)
		abs.Fragment = ""
		absStr := abs.String()

		linkDomain := strings.ToLower(abs.Hostname())
		isExternal := linkDomain != baseDomain && !allowedDomains[linkDomain]

		anchor := anchorWithContext(e)

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

	if result.TooLarge {
		return &result, fmt.Errorf("fetch %s: archivo de más de %d MB", pageURL, MaxDocumentBytes>>20)
	}

	// Portales de subastas que publican JSON (Granicus): se vuelve texto para
	// la huella de cambios; la extracción lee el JSON tal cual (granicus.go).
	if txt, ok := granicusText(result.RawHTML); ok {
		result.CleanText = txt
		result.ContentHash = scraper.HashContent(txt)
		return &result, nil
	}

	// Extract article content (strips nav, footer, sidebar, ads, scripts)
	if result.RawHTML != "" && !result.IsPDF {
		var meta scraper.ArticleMeta
		result.CleanText, meta = scraper.ExtractArticle(result.RawHTML)
		result.ContentHash = scraper.HashContent(result.CleanText)
		result.ImageURL = meta.Image
		result.Description = meta.Description
		result.PublishDate = meta.PublishDate
	}

	return &result, nil
}
