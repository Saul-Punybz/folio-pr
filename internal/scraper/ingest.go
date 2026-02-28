package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/ai"
	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/storage"
)

const (
	// maxDailyArticles is the maximum number of articles to ingest per day.
	maxDailyArticles = 500

	// maxConcurrentAI limits parallel AI enrichment goroutines.
	maxConcurrentAI = 3

	// defaultEvidencePolicy is the default retention policy for new articles.
	defaultEvidencePolicy = "ret_3m"
)

// DiscoveredArticle holds structured data from feed discovery. For RSS feeds,
// this includes the title, description, publish date, and image URL directly
// from the feed — avoiding the need to re-scrape the page for basic content.
type DiscoveredArticle struct {
	URL         string
	Title       string
	Description string
	Published   time.Time
	ImageURL    string
}

// Stores groups the data stores needed by the ingestion pipeline.
type Stores struct {
	Articles     *models.ArticleStore
	Sources      *models.SourceStore
	Fingerprints *models.FingerprintStore
}

// RunIngestion is the main ingestion job. It iterates over all active sources,
// discovers article URLs, deduplicates via fingerprints, scrapes content, and
// enqueues AI enrichment in background goroutines.
func RunIngestion(ctx context.Context, stores Stores, scraper *Scraper, aiClient *ai.OllamaClient, storageClient *storage.Client) {
	slog.Info("ingestion: starting run")
	startTime := time.Now()

	// Check how many articles we've already ingested today.
	todayCount, err := stores.Articles.CountToday(ctx)
	if err != nil {
		slog.Error("ingestion: count today", "err", err)
		todayCount = 0
	}

	remaining := maxDailyArticles - todayCount
	if remaining <= 0 {
		slog.Info("ingestion: daily limit reached", "count", todayCount)
		return
	}

	slog.Info("ingestion: daily budget", "used", todayCount, "remaining", remaining)

	// Load all active sources.
	sources, err := stores.Sources.ListActive(ctx)
	if err != nil {
		slog.Error("ingestion: list active sources", "err", err)
		return
	}

	if len(sources) == 0 {
		slog.Info("ingestion: no active sources configured")
		return
	}

	slog.Info("ingestion: processing sources", "count", len(sources))

	// Semaphore for concurrent AI enrichment.
	sem := make(chan struct{}, maxConcurrentAI)
	var wg sync.WaitGroup
	var ingested atomic.Int32

	for _, src := range sources {
		if ctx.Err() != nil {
			break
		}

		if int(ingested.Load()) >= remaining {
			slog.Info("ingestion: daily limit reached mid-run")
			break
		}

		discovered, err := discoverArticles(ctx, src, scraper)
		if err != nil {
			slog.Error("ingestion: discover articles",
				"source", src.Name,
				"feed_type", src.FeedType,
				"err", err,
			)
			continue
		}

		slog.Info("ingestion: discovered articles",
			"source", src.Name,
			"count", len(discovered),
		)

		for _, da := range discovered {
			if ctx.Err() != nil {
				break
			}

			if int(ingested.Load()) >= remaining {
				break
			}

			rawURL := da.URL

			// Canonicalize and check fingerprint.
			canonical := CanonicalizeURL(rawURL)
			urlHash := HashURL(rawURL)

			exists, blocked, err := stores.Fingerprints.ExistsOrBlocked(ctx, urlHash)
			if err != nil {
				slog.Error("ingestion: check fingerprint", "url", rawURL, "err", err)
				continue
			}
			if exists || blocked {
				slog.Debug("ingestion: skipping (fingerprint exists or blocked)",
					"url", rawURL,
					"exists", exists,
					"blocked", blocked,
				)
				continue
			}

			var title string
			var cleanText string
			var publishedAt time.Time
			var rawHTML string
			var imageURL string

			// If the discovered article has rich data from RSS, use it directly
			// instead of re-scraping the page (which often fails without selectors).
			if da.Description != "" {
				title = da.Title
				cleanText = da.Description
				publishedAt = da.Published
				imageURL = da.ImageURL

				slog.Debug("ingestion: using RSS feed data directly",
					"url", rawURL,
					"title", truncate(title, 60),
					"text_len", len(cleanText),
				)

				// Try to get og:image from the page if RSS didn't provide one.
				if imageURL == "" {
					imageURL = scraper.ExtractImageURL(ctx, rawURL)
				}
			} else {
				// No RSS data available — fall back to scraping the page.
				selectors := SourceSelectors{
					TitleSelector: src.TitleSelector,
					BodySelector:  src.BodySelector,
					DateSelector:  src.DateSelector,
				}

				scraped, scrapeErr := scraper.ScrapeArticle(ctx, rawURL, selectors)
				if scrapeErr != nil {
					slog.Error("ingestion: scrape article", "url", rawURL, "err", scrapeErr)
					continue
				}

				title = scraped.Title
				cleanText = scraped.CleanText
				publishedAt = scraped.PublishedAt
				rawHTML = scraped.RawHTML

				// Use RSS title/date as fallback if scraper didn't find them.
				if title == "" && da.Title != "" {
					title = da.Title
				}
				if publishedAt.IsZero() && !da.Published.IsZero() {
					publishedAt = da.Published
				}

				// Try to extract og:image from the raw HTML we already have.
				imageURL = extractOGImage(rawHTML)
				if imageURL == "" && da.ImageURL != "" {
					imageURL = da.ImageURL
				}
			}

			if title == "" && cleanText == "" {
				slog.Warn("ingestion: empty article, skipping", "url", rawURL)
				continue
			}

			// Filter out noise articles (Federal Register procedural filings, etc.)
			if isNoiseTitle(title) {
				slog.Debug("ingestion: skipping noise article", "title", truncate(title, 80), "url", rawURL)
				continue
			}

			// Create fingerprint record.
			contentHash := HashContent(cleanText)
			fp := &models.Fingerprint{
				CanonicalURLHash: urlHash,
				ContentHash:      contentHash,
			}
			if err := stores.Fingerprints.Create(ctx, fp); err != nil {
				slog.Error("ingestion: create fingerprint", "url", rawURL, "err", err)
				continue
			}

			// Determine evidence expiry based on policy.
			evidenceExpiry := evidenceExpiryTime(defaultEvidencePolicy)

			// Create the article record.
			article := &models.Article{
				ID:           uuid.New(),
				Title:        title,
				Source:       src.Name,
				URL:          rawURL,
				CanonicalURL: canonical,
				Region:       src.Region,
				PublishedAt:  timePtr(publishedAt),
				CleanText:    cleanText,
				ImageURL:     imageURL,
				Status:       "inbox",
				EvidencePolicy:    defaultEvidencePolicy,
				EvidenceExpiresAt: evidenceExpiry,
			}

			if err := stores.Articles.Create(ctx, article); err != nil {
				slog.Error("ingestion: create article", "url", rawURL, "err", err)
				continue
			}

			ingested.Add(1)
			slog.Info("ingestion: article created",
				"id", article.ID,
				"title", truncate(article.Title, 80),
				"source", src.Name,
				"has_image", imageURL != "",
			)

			// Enqueue AI enrichment in background.
			wg.Add(1)
			go func(art *models.Article, html string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				enrichArticle(ctx, art, html, stores, aiClient, storageClient)
			}(article, rawHTML)
		}
	}

	// Wait for all background AI enrichment to finish.
	wg.Wait()

	slog.Info("ingestion: run complete",
		"articles_ingested", ingested.Load(),
		"duration", time.Since(startTime).Round(time.Millisecond),
	)
}

// discoverArticles returns a list of discovered articles from a source based on
// its feed type. For RSS feeds, this includes structured data (title,
// description, date, image) directly from the feed items.
func discoverArticles(ctx context.Context, src models.Source, scraper *Scraper) ([]DiscoveredArticle, error) {
	switch src.FeedType {
	case "rss":
		if src.FeedURL == "" {
			return nil, fmt.Errorf("source %s: rss feed_url is empty", src.Name)
		}
		items, err := ParseFeed(ctx, src.FeedURL)
		if err != nil {
			return nil, err
		}
		results := make([]DiscoveredArticle, 0, len(items))
		for _, item := range items {
			if item.Link == "" {
				continue
			}
			da := DiscoveredArticle{
				URL:         item.Link,
				Title:       item.Title,
				Description: CleanText(item.Description),
				Published:   item.Published,
				ImageURL:    item.ImageURL,
			}
			results = append(results, da)
		}
		return results, nil

	case "scrape":
		if len(src.ListURLs) == 0 {
			return nil, fmt.Errorf("source %s: no list_urls configured", src.Name)
		}
		if src.LinkSelector == "" {
			return nil, fmt.Errorf("source %s: link_selector is empty", src.Name)
		}
		var results []DiscoveredArticle
		for _, listURL := range src.ListURLs {
			links, err := scraper.ScrapeLinks(ctx, listURL, src.LinkSelector)
			if err != nil {
				slog.Error("ingestion: scrape links", "list_url", listURL, "err", err)
				continue
			}
			for _, link := range links {
				results = append(results, DiscoveredArticle{URL: link})
			}
		}
		return results, nil

	case "sitemap":
		if src.FeedURL == "" {
			return nil, fmt.Errorf("source %s: sitemap feed_url is empty", src.Name)
		}
		urls, err := ParseSitemap(ctx, src.FeedURL)
		if err != nil {
			return nil, err
		}
		results := make([]DiscoveredArticle, 0, len(urls))
		for _, u := range urls {
			results = append(results, DiscoveredArticle{URL: u})
		}
		return results, nil

	default:
		return nil, fmt.Errorf("source %s: unsupported feed_type %q", src.Name, src.FeedType)
	}
}

// enrichArticle runs AI summarization, classification, entity extraction, and
// embedding, then uploads evidence to S3 and updates the article record.
func enrichArticle(ctx context.Context, article *models.Article, rawHTML string, stores Stores, aiClient *ai.OllamaClient, storageClient *storage.Client) {
	articleID := article.ID
	slog.Info("enrichment: starting", "id", articleID, "title", truncate(article.Title, 60))

	text := article.CleanText
	if text == "" {
		slog.Warn("enrichment: no clean text, skipping", "id", articleID)
		return
	}

	// Truncate very long texts for AI processing.
	aiText := text
	if len(aiText) > 8000 {
		aiText = aiText[:8000]
	}

	// Summarize.
	summary, err := aiClient.Summarize(ctx, aiText)
	if err != nil {
		slog.Error("enrichment: summarize", "id", articleID, "err", err)
	} else {
		slog.Debug("enrichment: summary generated", "id", articleID, "len", len(summary))
	}

	// Classify.
	tags, err := aiClient.Classify(ctx, aiText)
	if err != nil {
		slog.Error("enrichment: classify", "id", articleID, "err", err)
	} else {
		slog.Debug("enrichment: tags generated", "id", articleID, "tags", tags)
	}

	// Extract entities.
	entities, err := aiClient.ExtractEntities(ctx, aiText)
	if err != nil {
		slog.Error("enrichment: extract entities", "id", articleID, "err", err)
	} else {
		slog.Debug("enrichment: entities extracted", "id", articleID, "entities", entities)
	}

	// Generate embedding.
	embedding, err := aiClient.Embed(ctx, aiText)
	if err != nil {
		slog.Error("enrichment: embed", "id", articleID, "err", err)
	} else {
		slog.Debug("enrichment: embedding generated", "id", articleID)
	}

	// Update article with summary, tags, and embedding.
	if summary != "" || len(tags) > 0 || len(embedding) > 0 {
		if err := stores.Articles.UpdateEnrichment(ctx, articleID, summary, tags, embedding); err != nil {
			slog.Error("enrichment: update article", "id", articleID, "err", err)
		}
	}

	// Upload evidence to S3.
	if storageClient.Configured() {
		extracted, err := json.Marshal(map[string]interface{}{
			"title":    article.Title,
			"text":     article.CleanText,
			"tags":     tags,
			"entities": entities,
			"summary":  summary,
		})
		if err != nil {
			slog.Error("enrichment: marshal extracted", "id", articleID, "err", err)
		} else {
			policy := article.EvidencePolicy
			if policy == "" {
				policy = defaultEvidencePolicy
			}
			if err := storageClient.StoreEvidence(ctx, articleID, policy, []byte(rawHTML), extracted, nil); err != nil {
				slog.Error("enrichment: upload evidence", "id", articleID, "err", err)
			} else {
				slog.Debug("enrichment: evidence uploaded", "id", articleID)
			}
		}
	}

	slog.Info("enrichment: complete", "id", articleID)
}

// RunEvidenceCleanup deletes expired evidence from S3 and clears the expiry
// timestamp on the corresponding articles.
func RunEvidenceCleanup(ctx context.Context, stores Stores, storageClient *storage.Client) {
	slog.Info("evidence cleanup: starting")

	expired, err := stores.Articles.ListExpiredEvidence(ctx)
	if err != nil {
		slog.Error("evidence cleanup: list expired", "err", err)
		return
	}

	if len(expired) == 0 {
		slog.Info("evidence cleanup: no expired evidence")
		return
	}

	slog.Info("evidence cleanup: processing", "count", len(expired))

	cleaned := 0
	for _, article := range expired {
		if ctx.Err() != nil {
			break
		}

		// Delete from S3.
		if err := storageClient.DeleteEvidence(ctx, article.ID); err != nil {
			slog.Error("evidence cleanup: delete", "id", article.ID, "err", err)
			continue
		}

		// Clear the expiry timestamp.
		if err := stores.Articles.ClearEvidenceExpiry(ctx, article.ID); err != nil {
			slog.Error("evidence cleanup: clear expiry", "id", article.ID, "err", err)
			continue
		}

		cleaned++
	}

	slog.Info("evidence cleanup: complete", "cleaned", cleaned, "total", len(expired))
}

// RunSessionCleanup deletes expired sessions from the database.
func RunSessionCleanup(ctx context.Context, sessionStore *models.SessionStore) {
	slog.Info("session cleanup: starting")

	if err := sessionStore.DeleteExpired(ctx); err != nil {
		slog.Error("session cleanup: delete expired", "err", err)
		return
	}

	slog.Info("session cleanup: complete")
}

// evidenceExpiryTime calculates the evidence expiry time based on the policy.
func evidenceExpiryTime(policy string) *time.Time {
	now := time.Now().UTC()
	var expiry time.Time

	switch policy {
	case "ret_3m":
		expiry = now.AddDate(0, 3, 0)
	case "ret_6m":
		expiry = now.AddDate(0, 6, 0)
	case "ret_12m":
		expiry = now.AddDate(1, 0, 0)
	case "keep":
		return nil // Never expires.
	default:
		expiry = now.AddDate(0, 3, 0) // Default to 3 months.
	}

	return &expiry
}

// timePtr returns a pointer to the given time, or nil if it is the zero value.
func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// extractOGImage extracts the og:image or twitter:image content from raw HTML
// using simple string matching. This avoids an extra HTTP request when we
// already have the HTML from scraping.
func extractOGImage(html string) string {
	if html == "" {
		return ""
	}
	lower := strings.ToLower(html)

	// Try og:image.
	ogIdx := strings.Index(lower, `property="og:image"`)
	if ogIdx == -1 {
		ogIdx = strings.Index(lower, `property='og:image'`)
	}
	if ogIdx != -1 {
		if img := extractMetaContent(html, ogIdx); img != "" {
			return img
		}
	}

	// Fallback: twitter:image.
	twIdx := strings.Index(lower, `name="twitter:image"`)
	if twIdx == -1 {
		twIdx = strings.Index(lower, `name='twitter:image'`)
	}
	if twIdx != -1 {
		if img := extractMetaContent(html, twIdx); img != "" {
			return img
		}
	}

	return ""
}

// extractMetaContent finds the content="..." attribute value near the given
// position in HTML. It searches backward and forward within the enclosing
// <meta> tag.
func extractMetaContent(html string, attrIdx int) string {
	// Find the enclosing <meta tag — search backward for '<'.
	tagStart := strings.LastIndex(html[:attrIdx], "<")
	if tagStart == -1 {
		return ""
	}
	// Find the closing > of this tag.
	tagEnd := strings.Index(html[attrIdx:], ">")
	if tagEnd == -1 {
		return ""
	}
	tag := html[tagStart : attrIdx+tagEnd+1]
	tagLower := strings.ToLower(tag)

	// Find content="..."
	contentIdx := strings.Index(tagLower, `content="`)
	if contentIdx != -1 {
		start := contentIdx + len(`content="`)
		end := strings.Index(tagLower[start:], `"`)
		if end != -1 {
			return strings.TrimSpace(tag[start : start+end])
		}
	}

	contentIdx = strings.Index(tagLower, `content='`)
	if contentIdx != -1 {
		start := contentIdx + len(`content='`)
		end := strings.Index(tagLower[start:], `'`)
		if end != -1 {
			return strings.TrimSpace(tag[start : start+end])
		}
	}

	return ""
}

// noiseTitlePatterns are substrings (lowercased) that indicate an article is
// bureaucratic noise rather than real news. These are typically Federal Register
// procedural filings, requests for comments, administrative declarations, etc.
var noiseTitlePatterns = []string{
	"request for comments on the renewal",
	"request for comments on a previously",
	"administrative declaration of a disaster",
	"previously approved information collection",
	"renewal of a previously approved",
	"information collection request",
	"notice of proposed rulemaking",
	"proposed information collection",
	"agency information collection",
	"paperwork reduction act",
	"sunshine act meeting",
	"privacy act of 1974",
	"comment request",
	"60-day notice",
	"30-day notice",
	"submission for omb review",
}

// isNoiseTitle returns true if the article title matches common bureaucratic
// noise patterns that should be filtered out during ingestion.
func isNoiseTitle(title string) bool {
	lower := strings.ToLower(title)
	for _, pattern := range noiseTitlePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// truncate shortens a string to the given maximum length, appending "..." if
// truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
