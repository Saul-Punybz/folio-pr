package scraper

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// BingNewsSearch queries Bing News RSS and returns results as WebResult.
func BingNewsSearch(ctx context.Context, query string, limit int) ([]WebResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	feedURL := fmt.Sprintf("https://www.bing.com/news/search?q=%s&format=rss", url.QueryEscape(query))

	items, err := ParseFeed(ctx, feedURL)
	if err != nil {
		return nil, fmt.Errorf("bingsearch: %w", err)
	}

	var results []WebResult
	for _, item := range items {
		if len(results) >= limit {
			break
		}
		if item.Link == "" {
			continue
		}
		results = append(results, WebResult{
			Title:   item.Title,
			URL:     item.Link,
			Snippet: stripHTML(item.Description),
		})
	}

	return results, nil
}
