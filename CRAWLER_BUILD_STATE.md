# Crawler — Full UI + Integration Build State

## What Was Done

### Problem
The Folio crawler had a complete backend (16 endpoints, 7 tables, 3 cron jobs, AI enrichment, knowledge graph) but **ZERO frontend UI** and **broken integration** with the Research module. It was consuming resources (Ollama every 30min, crawling every 2h) with no user visibility.

### Solution: Option C — Full Implementation

| # | File | Status | Notes |
|---|------|--------|-------|
| 1 | `frontend/src/islands/CrawlerDesk.tsx` | DONE | 5-tab React island (Dashboard, Dominios, Páginas, Cambios, Grafo) |
| 2 | `frontend/src/pages/crawler.astro` | DONE | Astro page shell |
| 3 | `frontend/src/layouts/Layout.astro` | DONE | Added 'crawler' nav link |
| 4 | `frontend/src/lib/api.ts` | DONE | 7 interfaces + 16 API methods |
| 5 | `internal/research/phase1.go` | DONE | Added 7th search agent: crawl_index |
| 6 | `internal/handlers/crawler.go` | DONE | Fixed TriggerCrawl context bug |

## Architecture

### CrawlerDesk.tsx — 5 Tabs
1. **Dashboard**: Stats cards (pages/links/domains/queue), latest run info, queue status bars, "Iniciar Crawl" button, auto-refresh 30s
2. **Dominios**: Domain CRUD table with toggle active, add/edit/delete, category dropdown
3. **Páginas**: Page browser with search, domain filter, pagination, detail panel (entities, links, metadata)
4. **Cambios**: Recently changed pages with change summaries
5. **Grafo**: Entity network — grouped by type, relationship viewer with strength bars

### Research Integration
- Added `searchCrawlIndex()` as 7th agent in Phase 1 of research pipeline
- Uses existing `crawler.SearchCrawlIndex()` which was dead code
- Converts CrawledPage results to ResearchFinding records with source_type="crawl_index"
- research.Deps already had CrawledPages field — just needed to be wired

### Bug Fix: TriggerCrawl
- `r.Context()` was passed to background goroutine — gets cancelled when response sent
- Fixed: uses `context.Background()` with 2-hour timeout

## API Endpoints (existing backend, now with frontend)

| Method | Path | Action |
|--------|------|--------|
| GET | `/api/crawler/stats` | Aggregate stats |
| GET | `/api/crawler/runs` | Recent crawl runs |
| GET | `/api/crawler/queue` | Queue counts by status |
| GET | `/api/crawler/pages` | Paginated pages (filterable by domain) |
| GET | `/api/crawler/pages/search` | Full-text search pages |
| GET | `/api/crawler/pages/changes` | Recently changed pages |
| GET | `/api/crawler/pages/{id}` | Page detail + links + entities |
| GET | `/api/crawler/graph` | Knowledge graph (nodes + edges) |
| GET | `/api/crawler/graph/{entityId}/relations` | Entity relationships |
| GET | `/api/crawler/domains` | All domains |
| POST | `/api/crawler/domains` | Create domain (admin) |
| PUT | `/api/crawler/domains/{id}` | Update domain (admin) |
| PATCH | `/api/crawler/domains/{id}/toggle` | Toggle domain active (admin) |
| DELETE | `/api/crawler/domains/{id}` | Delete domain (admin) |
| POST | `/api/crawler/trigger` | Manual crawl trigger (admin) |

## Testing Steps

1. Build: `go build -o bin/api ./cmd/api && go build -o bin/worker ./cmd/worker`
2. Start API: `./bin/api` — serves on :8080
3. Navigate to `/crawler` — should show Dashboard tab
4. Check stats load (may be zeros if crawler hasn't run yet)
5. Go to Dominios tab — should show 15 preconfigured .gov.pr domains
6. Toggle a domain active/inactive
7. Go to Páginas tab — browse/search crawled pages
8. Click a page — see detail panel with entities, links
9. Go to Grafo tab — see entity network if enrichment has run
10. Click "Iniciar Crawl" — verify background crawl starts
11. Test Research integration: create a research project — Phase 1 should now include crawl_index results

## Next Steps (for new terminal session)

- [ ] Test Escritos feature end-to-end with Ollama running
- [ ] Test Crawler UI with live data
- [ ] Test Research + Crawler integration
- [ ] Consider Telegram bot commands for crawler (/crawl status, /crawl trigger)
- [ ] Deploy to Oracle VM
