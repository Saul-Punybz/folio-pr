// Package crawler implements the FolioBot web crawler that fetches, indexes,
// and indexes pages (change detection, no AI) from allowlisted government and news domains.
// También baja los PDF enlazados desde las páginas vigiladas, guarda una
// copia local y saca su texto, y de todo eso extrae «posibles fondos» por
// patrones (ver extraction.go). Nada usa IA.
package crawler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/pdftext"
	"github.com/Saul-Punybz/folio/internal/robots"
	"github.com/Saul-Punybz/folio/internal/scraper"
)

// Deps groups all dependencies needed by the crawler engine.
type Deps struct {
	Domains       *models.CrawlDomainStore
	Queue         *models.CrawlQueueStore
	Pages         *models.CrawledPageStore
	Links         *models.CrawlLinkStore
	Runs          *models.CrawlRunStore
	Opportunities *models.OpportunityStore
	Documents     *models.DocumentStore
	// DataDir: carpeta de datos; los PDF van a DataDir/documentos/<sha256>.pdf.
	DataDir string
	// Robots: robots.txt por host (si es nil, RunCrawl crea uno).
	Robots *robots.Checker
	// ForceEntries (botón «Buscar ahora»): vuelve a bajar y a revisar las
	// páginas de arranque de cada dominio aunque no les toque ni hayan cambiado.
	ForceEntries bool
}

const (
	crawlBatchSize = 10
	// minHostDelay: pausa mínima entre dos peticiones al mismo host.
	minHostDelay = time.Second
	// maxPDFLinksPerPage: una página con cientos de PDF (Caguas tiene 477)
	// no se come la corrida: se toman los primeros (suelen ser los recientes).
	maxPDFLinksPerPage = 40
)

// DocumentsDir es la carpeta de los PDF guardados.
func DocumentsDir(dataDir string) string { return filepath.Join(dataDir, "documentos") }

// CrawlStats resume una corrida (además de lo que queda en crawl_runs).
type CrawlStats struct {
	Pages, PDFReadable, PDFScanned, PDFRejected, RobotsSkipped int
}

// hostGate espacia las peticiones por host: mínimo 1 s o el Crawl-delay de robots.txt.
type hostGate struct {
	next map[string]time.Time
}

func (g *hostGate) wait(ctx context.Context, host string, delay time.Duration) error {
	if delay < minHostDelay {
		delay = minHostDelay
	}
	now := time.Now()
	if t, ok := g.next[host]; ok && t.After(now) {
		select {
		case <-time.After(t.Sub(now)):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	g.next[host] = time.Now().Add(delay)
	return nil
}

// RunCrawl is the main crawl job. It claims URLs from the queue, fetches them,
// extracts content + links, detects changes, and enqueues discovered links.
func RunCrawl(ctx context.Context, deps Deps, dailyBudget int) CrawlStats {
	var stats CrawlStats
	slog.Info("crawler: starting crawl", "budget", dailyBudget)

	if deps.Robots == nil {
		deps.Robots = robots.New(UserAgent, &http.Client{Timeout: 15 * time.Second})
	}
	gate := &hostGate{next: map[string]time.Time{}}

	if n, err := deps.Queue.ReleaseStale(ctx); err == nil && n > 0 {
		slog.Info("crawler: released stale queue items", "count", n)
	}

	// Create a run record
	run, err := deps.Runs.Create(ctx)
	if err != nil {
		slog.Error("crawler: create run", "err", err)
		return stats
	}

	// Build domain map for allowed domain checks
	domainMap, err := BuildDomainMap(ctx, deps.Domains)
	if err != nil {
		slog.Error("crawler: build domain map", "err", err)
		finishRun(ctx, deps, run, "failed", "build domain map: "+err.Error())
		return stats
	}

	// Build set for link discovery
	allowedSet := make(map[string]bool, len(domainMap))
	byID := make(map[uuid.UUID]*models.CrawlDomain, len(domainMap))
	for domain, d := range domainMap {
		allowedSet[domain] = true
		byID[d.ID] = d
	}

	// La URL de arranque de cada dominio activo (página de subastas/avisos).
	for _, d := range domainMap {
		entry := d.EntryURL()
		_ = deps.Queue.EnqueueEntry(ctx, &models.CrawlQueueItem{
			URL: entry, URLHash: scraper.HashURL(entry), DomainID: d.ID, Priority: d.Priority,
		}, deps.ForceEntries)
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
			isPDFItem := item.Kind == "pdf"

			// Las páginas tienen que ser de un dominio vigilado; los PDF
			// pueden estar en otro host del gobierno (se guardan con el
			// dominio de la página que los enlazó).
			domainID := item.DomainID
			domain := byID[item.DomainID]
			if !isPDFItem {
				id, allowed := IsAllowedDomain(item.URL, domainMap)
				if !allowed {
					_ = deps.Queue.MarkSkipped(ctx, item.ID)
					continue
				}
				domainID = id
				domain = byID[id]
				if domain != nil && item.Depth > domain.MaxDepth {
					_ = deps.Queue.MarkSkipped(ctx, item.ID)
					continue
				}
			} else if domain == nil {
				_ = deps.Queue.MarkSkipped(ctx, item.ID) // su dominio se apagó
				continue
			}

			// robots.txt antes de cada petición, y la pausa por host.
			if !deps.Robots.Allowed(ctx, item.URL) {
				_ = deps.Queue.MarkFailed(ctx, item.ID, "robots.txt no lo permite")
				stats.RobotsSkipped++
				continue
			}
			if err := gate.wait(ctx, DomainFromURL(item.URL), deps.Robots.CrawlDelay(item.URL)); err != nil {
				break
			}

			// Fetch page
			fetchCtx, fetchCancel := context.WithTimeout(ctx, 2*fetchTimeout)
			result, fetchErr := FetchPage(fetchCtx, item.URL, allowedSet)
			fetchCancel()
			totalCrawled++

			if fetchErr != nil {
				slog.Warn("crawler: fetch failed", "url", item.URL, "err", fetchErr)
				_ = deps.Queue.MarkFailed(ctx, item.ID, fetchErr.Error())
				run.PagesFailed++
				if isPDFItem {
					stats.PDFRejected++
				}
				continue
			}

			if result.StatusCode < 200 || result.StatusCode >= 400 {
				_ = deps.Queue.MarkFailed(ctx, item.ID, fmt.Sprintf("HTTP %d", result.StatusCode))
				run.PagesFailed++
				if isPDFItem {
					stats.PDFRejected++
				}
				continue
			}

			if result.IsPDF {
				isNew, err := savePDF(ctx, deps, item, domainID, result, &stats)
				if err != nil {
					slog.Warn("crawler: pdf rejected", "url", item.URL, "err", err)
					_ = deps.Queue.MarkFailed(ctx, item.ID, err.Error())
					run.PagesFailed++
					stats.PDFRejected++
					continue
				}
				if isNew {
					run.PagesNew++
				}
				domainsVisited[domainID] = true
				_ = deps.Queue.MarkDone(ctx, item.ID)
				run.PagesCrawled++
				continue
			}
			if isPDFItem {
				// El enlace decía .pdf pero llegó otra cosa (página de error, visor…).
				_ = deps.Queue.MarkFailed(ctx, item.ID, "no es un PDF: "+result.ContentType)
				run.PagesFailed++
				stats.PDFRejected++
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
				RawHTML:       result.RawHTML,
			}

			isNew, upsertErr := deps.Pages.Upsert(ctx, page)
			if upsertErr != nil {
				slog.Error("crawler: upsert page", "url", item.URL, "err", upsertErr)
				_ = deps.Queue.MarkFailed(ctx, item.ID, upsertErr.Error())
				run.PagesFailed++
				continue
			}

			if isNew {
				run.PagesNew++
				_ = deps.Domains.IncrementPageCount(ctx, domainID)
			} else if deps.ForceEntries && item.Depth == 0 && !page.Changed {
				_ = deps.Pages.MarkOpportunityUnchecked(ctx, page.ID)
			}
			if page.Changed {
				run.PagesChanged++
			}

			run.LinksDiscovered += recordLinks(ctx, deps, item, page, result.Links, domainMap)

			domainsVisited[domainID] = true
			_ = deps.Queue.MarkDone(ctx, item.ID)
			run.PagesCrawled++
			stats.Pages++
		}
	}

	run.DomainsVisited = len(domainsVisited)
	status := "completed"
	if ctx.Err() != nil {
		status = "stopped"
	}
	finishRun(context.WithoutCancel(ctx), deps, run, status, "")

	slog.Info("crawler: crawl complete",
		"pages_crawled", run.PagesCrawled,
		"pages_new", run.PagesNew,
		"pages_changed", run.PagesChanged,
		"pages_failed", run.PagesFailed,
		"links_discovered", run.LinksDiscovered,
		"domains_visited", run.DomainsVisited,
		"pdf_readable", stats.PDFReadable,
		"pdf_scanned", stats.PDFScanned,
		"pdf_rejected", stats.PDFRejected,
		"robots_skipped", stats.RobotsSkipped,
	)
	return stats
}

// recordLinks guarda los enlaces de una página y pone en cola los que se
// siguen: páginas del mismo grupo de dominios vigilados (respetando
// max_depth) y PDF, aunque estén en otro host del gobierno.
func recordLinks(ctx context.Context, deps Deps, item models.CrawlQueueItem, page *models.CrawledPage,
	links []DiscoveredLink, domainMap map[string]*models.CrawlDomain) int {
	_ = deps.Links.DeleteBySource(ctx, page.ID) // Clear old links on re-crawl
	var crawlLinks []models.CrawlLink
	seen := map[string]bool{}
	discovered, pdfs := 0, 0
	srcDomain := domainMap[DomainFromURL(item.URL)]
	if srcDomain == nil {
		for _, d := range domainMap {
			if d.ID == page.DomainID {
				srcDomain = d
				break
			}
		}
	}
	for _, link := range links {
		if seen[link.URL] {
			continue
		}
		seen[link.URL] = true
		linkHash := scraper.HashURL(link.URL)
		linkDomainID, linkAllowed := IsAllowedDomain(link.URL, domainMap)
		crawlLinks = append(crawlLinks, models.CrawlLink{
			SourcePageID: page.ID,
			TargetURL:    link.URL,
			TargetHash:   linkHash,
			AnchorText:   link.AnchorText,
			IsExternal:   !linkAllowed,
		})

		switch {
		case LooksLikePDF(link.URL):
			if pdfs >= maxPDFLinksPerPage || (!linkAllowed && !GovernmentHost(DomainFromURL(link.URL))) {
				continue
			}
			if srcDomain != nil && item.Depth+1 > srcDomain.MaxDepth+1 {
				continue
			}
			pdfs++
			_ = deps.Queue.Enqueue(ctx, &models.CrawlQueueItem{
				URL: link.URL, URLHash: linkHash, DomainID: page.DomainID,
				Depth: item.Depth + 1, Priority: 10, DiscoveredBy: &page.ID,
				Kind: "pdf", AnchorText: link.AnchorText,
			})
			discovered++
		case linkAllowed && followable(link.URL):
			d := domainMap[DomainFromURL(link.URL)]
			if d == nil {
				for _, cand := range domainMap {
					if cand.ID == linkDomainID {
						d = cand
						break
					}
				}
			}
			if d != nil && item.Depth+1 > d.MaxDepth {
				continue
			}
			prio := item.Priority
			if d != nil {
				prio = d.Priority
			}
			_ = deps.Queue.Enqueue(ctx, &models.CrawlQueueItem{
				URL: link.URL, URLHash: linkHash, DomainID: linkDomainID,
				Depth: item.Depth + 1, Priority: prio, DiscoveredBy: &page.ID,
			})
			discovered++
		}
	}
	if len(crawlLinks) > 0 {
		_ = deps.Links.BulkInsert(ctx, crawlLinks)
	}
	return discovered
}

// savePDF guarda el original en <datos>/documentos/<sha256>.pdf, saca el
// texto y registra el PDF como página (para la extracción y la búsqueda).
func savePDF(ctx context.Context, deps Deps, item models.CrawlQueueItem, domainID uuid.UUID,
	res *FetchResult, stats *CrawlStats) (bool, error) {
	if deps.Documents == nil || deps.DataDir == "" {
		return false, errors.New("sin carpeta de documentos")
	}
	if len(res.Body) > MaxDocumentBytes {
		return false, fmt.Errorf("PDF de más de %d MB", MaxDocumentBytes>>20)
	}
	text, err := pdftext.Extract(res.Body)
	if err != nil {
		return false, err
	}
	sum := sha256.Sum256(res.Body)
	sha := hex.EncodeToString(sum[:])
	rel := filepath.Join("documentos", sha+".pdf")
	abs := filepath.Join(deps.DataDir, rel)
	if _, statErr := os.Stat(abs); statErr != nil {
		if err := os.MkdirAll(DocumentsDir(deps.DataDir), 0o700); err != nil {
			return false, err
		}
		tmp := abs + ".tmp"
		if err := os.WriteFile(tmp, res.Body, 0o600); err != nil {
			return false, err
		}
		if err := os.Rename(tmp, abs); err != nil {
			return false, err
		}
	}
	doc := &models.Document{
		SHA256: sha, Path: rel, SizeBytes: int64(len(res.Body)), Pages: text.Pages,
		Scanned: text.Scanned, SourceURL: item.URL,
	}
	if err := deps.Documents.Upsert(ctx, doc); err != nil {
		return false, err
	}
	if text.Scanned {
		stats.PDFScanned++
	} else {
		stats.PDFReadable++
	}

	title := strings.TrimSpace(item.AnchorText)
	if title == "" {
		title = pdfFileTitle(item.URL)
	}
	page := &models.CrawledPage{
		URL:           item.URL,
		URLHash:       scraper.HashURL(item.URL),
		DomainID:      domainID,
		Title:         title,
		CleanText:     text.Text,
		ContentHash:   sha,
		Depth:         item.Depth,
		StatusCode:    res.StatusCode,
		ContentType:   "application/pdf",
		ContentLength: len(res.Body),
		DocumentID:    &doc.ID,
		AnchorText:    item.AnchorText,
		SourcePageID:  item.DiscoveredBy,
	}
	return deps.Pages.Upsert(ctx, page)
}

// pdfFileTitle: nombre del archivo, legible ("Aviso-RFP-S-2027_018").
func pdfFileTitle(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	name, _ := url.PathUnescape(path.Base(u.Path))
	return strings.TrimSuffix(strings.TrimSuffix(name, ".pdf"), ".PDF")
}

// LooksLikePDF: el enlace termina en .pdf (antes de ?/#).
func LooksLikePDF(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return strings.HasSuffix(strings.ToLower(u.Path), ".pdf")
}

// GovernmentHost: hosts del gobierno (PR, municipios y federal).
func GovernmentHost(host string) bool {
	host = strings.ToLower(host)
	return strings.HasSuffix(host, ".gov") || strings.HasSuffix(host, ".gov.pr") ||
		strings.HasSuffix(host, ".pr.gov") || strings.HasSuffix(host, ".mil")
}

// skipExt: archivos que no son páginas (se registran como enlace, no se bajan).
var skipExt = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".svg": true, ".webp": true, ".ico": true,
	".css": true, ".js": true, ".zip": true, ".rar": true, ".mp3": true, ".mp4": true, ".mov": true,
	".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".ppt": true, ".pptx": true, ".xml": true,
	".json": true, ".ics": true, ".dwg": true, ".exe": true, ".dmg": true,
}

func followable(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	return !skipExt[strings.ToLower(path.Ext(u.Path))]
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

func finishRun(ctx context.Context, deps Deps, run *models.CrawlRun, status, errMsg string) {
	now := time.Now()
	run.Status = status
	run.ErrorMsg = errMsg
	run.FinishedAt = &now
	if updateErr := deps.Runs.Update(ctx, run); updateErr != nil {
		slog.Error("crawler: update run", "err", updateErr)
	}
}
