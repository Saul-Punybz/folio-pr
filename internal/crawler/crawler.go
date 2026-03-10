// Package crawler implements the FolioBot web crawler that fetches, indexes,
// and enriches pages from allowlisted government and news domains.
package crawler

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/ai"
	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/scraper"
)

// Deps groups all dependencies needed by the crawler engine.
type Deps struct {
	Domains  *models.CrawlDomainStore
	Queue    *models.CrawlQueueStore
	Pages    *models.CrawledPageStore
	Links    *models.CrawlLinkStore
	Runs     *models.CrawlRunStore
	Entities *models.EntityStore
	PageEnts *models.PageEntityStore
	Rels     *models.EntityRelationshipStore
	AI       *ai.OllamaClient
}

const (
	crawlBatchSize = 10
)

// RunCrawl is the main crawl job. It claims URLs from the queue, fetches them,
// extracts content + links, detects changes, and enqueues discovered links.
func RunCrawl(ctx context.Context, deps Deps, dailyBudget int) {
	slog.Info("crawler: starting crawl", "budget", dailyBudget)

	// Create a run record
	run, err := deps.Runs.Create(ctx)
	if err != nil {
		slog.Error("crawler: create run", "err", err)
		return
	}

	// Build domain map for allowed domain checks
	domainMap, err := BuildDomainMap(ctx, deps.Domains)
	if err != nil {
		slog.Error("crawler: build domain map", "err", err)
		finishRun(ctx, deps, run, "failed", "build domain map: "+err.Error())
		return
	}

	// Build set for link discovery
	allowedSet := make(map[string]bool, len(domainMap))
	for domain := range domainMap {
		allowedSet[domain] = true
	}

	// Tracking
	domainsVisited := make(map[uuid.UUID]bool)
	totalCrawled := 0

	for totalCrawled < dailyBudget {
		if ctx.Err() != nil {
			slog.Info("crawler: context cancelled, stopping")
			break
		}

		batchSize := crawlBatchSize
		remaining := dailyBudget - totalCrawled
		if remaining < batchSize {
			batchSize = remaining
		}

		batch, claimErr := deps.Queue.ClaimBatch(ctx, batchSize)
		if claimErr != nil {
			slog.Error("crawler: claim batch", "err", claimErr)
			break
		}
		if len(batch) == 0 {
			slog.Info("crawler: queue empty, stopping")
			break
		}

		for _, item := range batch {
			if ctx.Err() != nil {
				break
			}

			// Check domain is still allowed
			domainID, allowed := IsAllowedDomain(item.URL, domainMap)
			if !allowed {
				_ = deps.Queue.MarkSkipped(ctx, item.ID)
				continue
			}

			// Check depth limit
			domain := domainMap[DomainFromURL(item.URL)]
			if domain == nil {
				// Try with and without www
				for _, d := range domainMap {
					if d.ID == domainID {
						domain = d
						break
					}
				}
			}
			if domain != nil && item.Depth > domain.MaxDepth {
				_ = deps.Queue.MarkSkipped(ctx, item.ID)
				continue
			}

			// Fetch page
			fetchCtx, fetchCancel := context.WithTimeout(ctx, 30*time.Second)
			result, fetchErr := FetchPage(fetchCtx, item.URL, allowedSet)
			fetchCancel()

			if fetchErr != nil {
				slog.Warn("crawler: fetch failed", "url", item.URL, "err", fetchErr)
				_ = deps.Queue.MarkFailed(ctx, item.ID, fetchErr.Error())
				run.PagesFailed++
				totalCrawled++
				continue
			}

			if result.StatusCode < 200 || result.StatusCode >= 400 {
				_ = deps.Queue.MarkFailed(ctx, item.ID, "HTTP "+string(rune(result.StatusCode+'0')))
				run.PagesFailed++
				totalCrawled++
				continue
			}

			// Calculate next crawl time
			recrawlHours := 168
			if domain != nil {
				recrawlHours = domain.RecrawlHours
			}
			nextCrawl := ScheduleNextCrawl(recrawlHours)

			// Upsert crawled page
			page := &models.CrawledPage{
				URL:           item.URL,
				URLHash:       scraper.HashURL(item.URL),
				DomainID:      domainID,
				Title:         result.Title,
				CleanText:     result.CleanText,
				ContentHash:   result.ContentHash,
				LinksOut:      len(result.Links),
				Depth:         item.Depth,
				StatusCode:    result.StatusCode,
				ContentType:   result.ContentType,
				ContentLength: result.ContentLength,
				NextCrawlAt:   &nextCrawl,
			}

			isNew, upsertErr := deps.Pages.Upsert(ctx, page)
			if upsertErr != nil {
				slog.Error("crawler: upsert page", "url", item.URL, "err", upsertErr)
				_ = deps.Queue.MarkFailed(ctx, item.ID, upsertErr.Error())
				run.PagesFailed++
				totalCrawled++
				continue
			}

			if isNew {
				run.PagesNew++
				_ = deps.Domains.IncrementPageCount(ctx, domainID)
			}
			if page.Changed {
				run.PagesChanged++
			}

			// Record links
			_ = deps.Links.DeleteBySource(ctx, page.ID) // Clear old links on re-crawl
			var crawlLinks []models.CrawlLink
			for _, link := range result.Links {
				linkHash := scraper.HashURL(link.URL)
				crawlLinks = append(crawlLinks, models.CrawlLink{
					SourcePageID: page.ID,
					TargetURL:    link.URL,
					TargetHash:   linkHash,
					AnchorText:   link.AnchorText,
					IsExternal:   link.IsExternal,
				})

				// Enqueue internal links for crawling
				if !link.IsExternal {
					linkDomainID, linkAllowed := IsAllowedDomain(link.URL, domainMap)
					if linkAllowed {
						_ = deps.Queue.Enqueue(ctx, &models.CrawlQueueItem{
							URL:          link.URL,
							URLHash:      linkHash,
							DomainID:     linkDomainID,
							Depth:        item.Depth + 1,
							Priority:     item.Priority,
							DiscoveredBy: &page.ID,
						})
						run.LinksDiscovered++
					}
				}
			}
			if len(crawlLinks) > 0 {
				_ = deps.Links.BulkInsert(ctx, crawlLinks)
			}

			domainsVisited[domainID] = true
			_ = deps.Queue.MarkDone(ctx, item.ID)
			run.PagesCrawled++
			totalCrawled++
		}
	}

	run.DomainsVisited = len(domainsVisited)
	status := "completed"
	if ctx.Err() != nil {
		status = "stopped"
	}
	finishRun(ctx, deps, run, status, "")

	slog.Info("crawler: crawl complete",
		"pages_crawled", run.PagesCrawled,
		"pages_new", run.PagesNew,
		"pages_changed", run.PagesChanged,
		"pages_failed", run.PagesFailed,
		"links_discovered", run.LinksDiscovered,
		"domains_visited", run.DomainsVisited,
	)
}

// RunRecrawl re-enqueues pages that are past their next_crawl_at timestamp.
func RunRecrawl(ctx context.Context, deps Deps) {
	slog.Info("crawler: starting recrawl enqueue")

	count, err := deps.Queue.ReenqueueForRecrawl(ctx)
	if err != nil {
		slog.Error("crawler: recrawl enqueue", "err", err)
		return
	}

	slog.Info("crawler: recrawl enqueue complete", "enqueued", count)
}

// RunEnrichment runs AI enrichment on un-enriched crawled pages.
func RunEnrichment(ctx context.Context, deps Deps, batchSize int) {
	if batchSize <= 0 {
		batchSize = 50
	}

	slog.Info("crawler: starting enrichment", "batch_size", batchSize)

	pages, err := deps.Pages.ListUnenriched(ctx, batchSize)
	if err != nil {
		slog.Error("crawler: list unenriched", "err", err)
		return
	}

	if len(pages) == 0 {
		slog.Debug("crawler: no pages to enrich")
		return
	}

	enriched := 0
	for i := range pages {
		if ctx.Err() != nil {
			break
		}

		if err := EnrichPage(ctx, deps, &pages[i]); err != nil {
			slog.Warn("crawler: enrich page", "id", pages[i].ID, "err", err)
			continue
		}
		enriched++
	}

	slog.Info("crawler: enrichment complete", "enriched", enriched, "total", len(pages))
}

func finishRun(ctx context.Context, deps Deps, run *models.CrawlRun, status, errMsg string) {
	now := time.Now()
	run.Status = status
	run.ErrorMsg = errMsg
	run.FinishedAt = &now
	if updateErr := deps.Runs.Update(ctx, run); updateErr != nil {
		slog.Error("crawler: update run", "err", updateErr)
	}
}
