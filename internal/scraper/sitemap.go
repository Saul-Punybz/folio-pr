package scraper

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// sitemapURLSet is the root element of a sitemap.xml file.
type sitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

// ParseSitemap fetches and parses a sitemap.xml file, returning the list of
// URLs found.
func ParseSitemap(ctx context.Context, sitemapURL string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sitemapURL, nil)
	if err != nil {
		return nil, fmt.Errorf("sitemap: create request: %w", err)
	}
	req.Header.Set("User-Agent", feedUserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sitemap: fetch %s: %w", sitemapURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sitemap: fetch %s: status %d", sitemapURL, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("sitemap: read body: %w", err)
	}

	var urlSet sitemapURLSet
	if err := xml.Unmarshal(body, &urlSet); err != nil {
		return nil, fmt.Errorf("sitemap: parse %s: %w", sitemapURL, err)
	}

	urls := make([]string, 0, len(urlSet.URLs))
	for _, u := range urlSet.URLs {
		loc := strings.TrimSpace(u.Loc)
		if loc != "" {
			urls = append(urls, loc)
		}
	}

	return urls, nil
}
