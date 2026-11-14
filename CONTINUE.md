# Folio -- Continuation Guide

## Quick Resume

Say **"CONTINUE FOLIO"** to pick up where we left off.

## How to Run (macOS App)

```bash
# Option A: Launch the built .app (self-contained, no setup needed)
open /Users/saulgonzalez/Downloads/folio/build/Folio.app

# Option B: Rebuild from source then launch
cd /Users/saulgonzalez/Downloads/folio
bash scripts/build-macos.sh
open build/Folio.app
```

The app bundles PostgreSQL, auto-runs migrations, starts ingestion, and opens `http://localhost:8080` in your browser. No login required.

**Requirements:** Ollama must be running (`ollama serve`) with models `llama3.2:3b` and `nomic-embed-text`.

### Developer Mode (separate processes)

```bash
cd /Users/saulgonzalez/Downloads/folio

# Build
go build -o bin/api ./cmd/api && go build -o bin/worker ./cmd/worker
cd frontend && npm run build && cd ..

# Run (from project root!)
./bin/api    # API + frontend on :8080
./bin/worker # Background cron
```

## Current State (Mar 11, 2026)

### Session Mar 11 — Crawler → Grant/RFP Opportunity Tracker

**Transformed the Crawler section** from a generic page browser into a **focused grant/RFP opportunity tracker**. The crawler visits government agency websites and the AI (llama3.2:3b) extracts structured opportunity data.

**Backend — New files:**
- `migrations/020_opportunities.sql` — `opportunities` table (title, agency, amount, deadline, contact, category, status, confidence, source_url, etc.) + `opportunity_checked` column on `crawled_pages`
- `internal/models/opportunity.go` — `OpportunityStore` with Upsert, ListAll (filters: status/category/domain + sort), SearchFTS, CountByStatus, RefreshStatuses, MarkReviewed, etc.
- `internal/handlers/opportunities.go` — 9 endpoints: list, stats, search, get, update, delete, review, refresh-statuses, extract

**Backend — Edited files:**
- `internal/models/crawled_page.go` — Added `OpportunityChecked` field, `MarkOpportunityChecked()`, `ListUncheckedForOpportunities()`. Updated ALL SELECT/Scan calls.
- `internal/ai/ollama.go` — Added `ExtractedOpportunity` struct + `ExtractOpportunity()` method with structured AI prompt
- `internal/crawler/crawler.go` — Added `Opportunities *models.OpportunityStore` to `Deps`
- `internal/crawler/enrich.go` — Added opportunity extraction pipeline: keyword pre-filter (EN+ES terms) → AI extraction → parse deadline → compute status (open/closing_soon/closed) → upsert. Added `RunOpportunityExtraction()` for batch processing.
- `cmd/app/main.go` — Wired OpportunityStore, handler, routes (`/api/opportunities/*`), 2 cron jobs (extraction every 30min, status refresh daily 6am)
- `frontend/src/lib/api.ts` — Added `Opportunity` interface + 9 API methods

**Frontend — `CrawlerDesk.tsx` (rewritten 3 times based on feedback):**
- Final design: **2 tabs** (Oportunidades + Dominios)
- **One "Escanear" button** in the header — triggers crawl → extraction → shows results (single action)
- **All process state lives at the parent level** — tab switching does NOT kill running scans
- **Global progress bar** visible on both tabs, shows phase (1/2 Crawl, 2/2 AI) + checkmark when done
- **Domain sidebar filter** on left side of Oportunidades tab — click agency to filter results
- **Structured opportunity cards** with category badge, status badge (pulsing for closing_soon), confidence %, amount, deadline countdown, contact info, source link
- **Smart empty states** — guides user to add domains or click Escanear

**User feedback addressed:**
1. Initial 3-tab design had action buttons buried in "Configuracion" (3rd tab) → Removed tab, put buttons in header
2. Tab switching killed the crawl process → Lifted state to parent component
3. Two separate buttons (crawl + extract) confused the user → Merged into one "Escanear" button

**Bugs found and fixed (same session):**
1. **Old `api` binary on port 8080** was blocking the Folio.app — had to `kill -9` the old process
2. **Opportunity routes returned 404** — macOS `open` doesn't restart running apps; had to kill + relaunch manually
3. **Extraction query required `enriched = true`** but 0 pages were enriched → Changed to `length(clean_text) > 100`
4. **Crawl run counter never updated** (always showed 0 pages) → Frontend waited forever for "completed"
5. **Frontend flow was sequential** (wait for crawl → then extract) → Changed to **parallel** (trigger both, poll for results)
6. **External links don't work in Wails WebView** → Added `openExternal()` using `window.runtime.BrowserOpenURL()`
7. **Cards had no expand/read** → Added expandable cards with full page content reader

**Migration 021 — PR Agency Domains:**
Added 21 new government agency domains: subastas.pr.gov, recovery.pr.gov, recuperacion.pr.gov, fondosfederales.pr.gov, asg.pr.gov, desarrollo.pr.gov, cdbg-dr.pr.gov, prits.pr.gov, familia.pr.gov, agricultura.pr.gov, tourism.pr.gov, bde.pr.gov, energia.pr.gov, smartisland.pr.gov, p3.pr.gov, prpa.pr.gov, lumapr.com, acueductos.pr.gov, puertoricobids.net, puertorico.grantwatch.com, jp.pr.gov

**Verified working:**
- 4 opportunities extracted from existing 480 pages (DRNA, Hacienda, Trabajo)
- AI extraction pipeline works end-to-end via Ollama
- Opportunity cards show in UI with badges, expandable with full page content

**Next steps:**
- Run full scan with all 36 domains to get more opportunities
- The `clean_text` has HTML artifacts (CSS, JS mixed in) — consider improving text extraction in the crawler
- Test "Abrir en Navegador" button in Wails
- Consider adding a sidebar rename for "Crawler" → "Oportunidades" in the Layout.astro sidebar

### Previous: Session Mar 10 — Database Fixes + Re-Enrichment Pipeline

### Session Mar 10 (evening) — Database Fixes + Re-Enrichment Pipeline

**1. Fixed duplicate sources (migration 019)**
- Migration 018 had created 21 duplicate sources (88 rows → 67 unique)
- `019_fix_sources_link_articles.sql` — Deduplicates using `DISTINCT ON (name, region)`, links all orphaned articles

**2. Linked all orphaned articles to their sources**
- All 2,654 articles had `source_id = NULL` — now 2,646 are linked (8 "manual" entries have no source)
- Added `SourceID *uuid.UUID` field to Article model
- Updated `Create()` to include `source_id` in INSERT
- Updated ingestion pipeline (`ingest.go`) to set `SourceID` from the source being processed

**3. Re-enrichment pipeline**
- Added `ListNeedingReEnrichment()` to ArticleStore — finds articles missing embeddings or sentiment
- Added `RunReEnrichment()` to scraper package — processes batch using full enrichment pipeline (summary, tags, entities, sentiment, embedding)
- Worker cron: runs every hour at :30, processes 50 articles per batch
- Admin API: `POST /api/admin/reenrich` triggers immediate batch of 100
- Updated admin handler to use full enrichment pipeline (was missing entity extraction + sentiment)
- Added `EntityStore` to AdminHandler struct and all instantiation sites

**4. Current enrichment status**
- 2,654 total articles, 2,646 linked to sources
- ~1,540 have embeddings (was 1,486), ~1,510 have sentiment (was 1,456)
- Re-enrichment running in background, ~6 articles/30s with 3 concurrent workers

**5. Sources: 67 total (56 active)**
- 38 PR, 15 US, 2 Federal, 1 Grants active
- 11 inactive (broken feeds)

### Previous Session — macOS App + Full Media Directory + PR Filter

**1. Folio.app — Self-contained macOS application**
- `cmd/app/main.go` — Single binary that bundles everything: embedded PostgreSQL, auto-migrations, API server, frontend serving, background worker cron, auto-opens browser
- `scripts/build-macos.sh` — Builds Folio.app bundle with bundled PG, creates DMG
- App data lives at `~/Library/Application Support/Folio/` (pgdata, logs)
- PostgreSQL runs on a random port (check `ps aux | grep postgres.*Folio`)
- No login required — uses AutoAuth middleware that auto-creates admin user

**2. Login removed**
- Deleted: `frontend/src/pages/login.astro`, `frontend/src/islands/LoginForm.tsx`
- Removed: logout button from Layout.astro, login/logout API endpoints from cmd/app/main.go
- Removed: 401→login redirect from api.ts, api.login() function

**3. macOS App Icon**
- `scripts/AppIcon.icns` — Violet rounded rectangle with white bold "F"
- Generated via Python Pillow → iconset → `iconutil -c icns`

**4. Complete Media Directory (44 sources)**
- Migration `018_complete_directory.sql` — Inserts all missing sources with verified working RSS feeds
- **21 active PR sources:** El Nuevo Dia, Primera Hora, Metro PR, El Vocero, CPI, Telemundo, NotiCel, Radio Isla, TeleOnce, WIPR, Es Noticia PR, News is My Business, Hacia Donde Vamos, Puestos Pa'l Problema, Colmena66, Inprende, PR Science Trust, Foundation for Puerto Rico
- **14 active US sources:** CNN, NYT, Washington Post, WSJ, Bloomberg, NBC, ABC, CBS, Fox News, Forbes, TechCrunch, Harvard Business Review, Y Combinator News, The Rundown AI
- **3 active Federal/Grants:** Federal Register, GovInfo, GAO Reports
- **9 deactivated (broken feeds):** WAPA.TV, Noticentro, El Calce, Caribbean Business, Univision, Jay Fonseca, La Fortaleza, Senado, Camara de Representantes

**5. PR-Relevance Filter for US articles**
- Added to `internal/scraper/ingest.go`:
  - `prKeywords` — comprehensive list of PR-related terms (puerto rico, boricua, san juan, jones act, act 60, junta de control fiscal, promesa, city names, etc.)
  - `isPRRelevant()` — fast keyword match against article text
  - `aiCheckPRRelevance()` — Ollama-based yes/no classification fallback
- Flow: US/Federal/Global articles → keyword check → if no match, Ollama AI check → skip if not PR-related
- PR and Grants region articles bypass the filter entirely

**6. Frontend serving fix**
- `cmd/app/main.go` wildcard handler: serves Astro `directory/index.html` files directly via `io.Copy` to avoid `http.FileServer`'s redirect loop (`/inbox/index.html` → `/inbox/` → loop)

### Migrations (20 total)

| # | File | Description |
|---|------|-------------|
| 001 | init.sql | Core schema (articles, sources, users, sessions, fingerprints) |
| 002 | seed_sources.sql | 24 initial PR + Grants sources |
| 003-006 | various | Phase 2-3 enhancements, image_url, regions |
| 007 | watchlist.sql | Watchlist orgs + hits |
| 008 | feed_token.sql | Feed token for sources |
| 009 | chat_sessions.sql | AI chat session persistence |
| 010 | entities.sql | Entity graph + sentiment |
| 011 | telegram.sql | Telegram users + notifications |
| 012 | research.sql | Deep research projects + findings |
| 013 | crawler.sql | Web crawler tables |
| 014 | escritos.sql | AI-generated articles |
| 015 | unaccent.sql | PostgreSQL unaccent extension |
| 016 | media_directory.sql | Expand region constraint (US, Global) |
| 017 | activate_directory.sql | Activate/convert existing sources to RSS |
| 018 | complete_directory.sql | INSERT all missing US/PR sources with verified RSS feeds |
| 019 | fix_sources_link_articles.sql | Deduplicate sources, link orphaned articles |
| 020 | opportunities.sql | Opportunities table + opportunity_checked on crawled_pages |

### RSS Feed Status (verified Mar 10, 2026)

**Working feeds:**
- El Nuevo Dia, Primera Hora, Metro PR, El Vocero, CPI, Telemundo, NotiCel, Radio Isla, TeleOnce, WIPR, Es Noticia PR, News is My Business
- Hacia Donde Vamos (podcast), Puestos Pa'l Problema (podcast)
- Colmena66, Inprende, PR Science Trust, Foundation for PR
- All 14 US news sources (CNN, NYT, WashPost, WSJ, Bloomberg, NBC, ABC, CBS, Fox, Forbes, TechCrunch, HBR, YC News, Rundown AI)
- Federal Register (Atom, PR-specific search), GovInfo, GAO Reports

**Broken (deactivated):**
- WAPA.TV / Noticentro: 429 rate limited
- El Calce: 404
- Caribbean Business: timeout
- Univision PR: 404
- Jay Fonseca: 500 (Next.js error)
- La Fortaleza, Senado, Camara: 404 (government sites have no RSS)

### What Works
- **macOS App**: Double-click Folio.app → everything runs, browser opens
- **67 sources** (56 active): 38 PR + 15 US + 2 Federal + 1 Grants
- **PR-relevance filter**: US articles checked for Puerto Rico connection
- **No login**: Direct access, auto-auth
- **Articles linked to sources**: 2,646/2,654 have source_id (8 manual entries)
- **Re-enrichment pipeline**: Hourly cron + admin API trigger for missing embeddings/sentiment
- **All previous features**: Inbox, saved, archive, search, AI chat, daily brief, watchlist, analytics, research, crawler, escritos, Telegram bot, entity graph, sentiment analysis
- **Keyboard shortcuts**: J/K navigate, S save, T trash, P pin, Enter expand, ESC close

### What Needs Work

#### High Priority
1. ~~Test full ingestion cycle~~ DONE — ingestion works, all items fingerprinted
2. ~~Re-enrich existing articles~~ DONE — pipeline running, processing ~6 articles/30s
3. ~~Fix orphaned articles~~ DONE — migration 019, 2,646/2,654 linked

#### Medium Priority
4. **Deploy to Oracle VM** — `scripts/deploy-oracle.sh` exists but untested
5. **Test Telegram bot** — Set up BotFather token, test all commands
6. **Reactivate broken sources** — Find working feeds for WAPA, Caribbean Business, Univision, etc.
7. **Evidence retention lifecycle** — Needs real S3 for archive
8. **Pagination** — Inbox loads all articles; add infinite scroll

#### Polish
9. **Dark mode refinement** — Some elements need tuning
10. **Mobile responsive** — Grid works but action buttons could improve
11. **Reading time estimate** — Calculate from clean_text word count

## Architecture

```
cmd/app/main.go          -- macOS app: embedded PG + API + worker + browser (all-in-one)
cmd/api/main.go          -- Standalone API server (Chi router, serves frontend)
cmd/worker/main.go       -- Standalone background worker (cron: ingestion, cleanup, briefs)
cmd/bot/main.go          -- Standalone Telegram bot
internal/
  ai/ollama.go           -- Ollama client (summarize, embed, generate, entities, sentiment)
  config/                -- Environment config loader
  db/                    -- PostgreSQL connection (pgx), auto-migrations
  intelligence/          -- Reusable chat logic
  telegram/              -- Telegram bot (commands, callbacks, notifications)
  research/              -- Deep research engine (3-phase: search → scrape → synthesize)
  handlers/              -- HTTP handlers (items, search, notes, briefs, watchlist, chat, sources, admin, analytics, research, crawler, escritos, opportunities)
  middleware/            -- Auth middleware
  models/               -- DB models (article, source, user, session, note, brief, entity, research, crawler, escrito)
  scraper/              -- RSS parser, Colly scraper, ingestion pipeline, PR-relevance filter
  storage/              -- S3 client for evidence archival
  generator/            -- Escritos AI article generator
frontend/
  src/islands/          -- React interactive components
  src/lib/api.ts        -- Frontend API client
  src/layouts/          -- Astro layout with vertical violet sidebar
  src/pages/            -- 11 pages (inbox, saved, archive, brief, watchlist, settings, analytics, research, crawler, domains, escritos)
scripts/
  build-macos.sh        -- Build Folio.app bundle
  AppIcon.icns          -- macOS app icon
migrations/             -- 20 SQL migrations
```

## Database Access (App's bundled PG)

```bash
# Find the port (changes each launch)
ps aux | grep 'postgres.*Folio' | grep -oP '\-p \K[0-9]+'

# Connect
/opt/homebrew/Cellar/postgresql@17/17.9/bin/psql -U folio -d folio -p <PORT> -h localhost

# Useful queries
SELECT source, region, COUNT(*) FROM articles GROUP BY source, region ORDER BY count DESC;
SELECT name, feed_type, region, active FROM sources ORDER BY region, active DESC, name;
SELECT COUNT(*), MAX(created_at) FROM articles;
```

## Design

- **Vertical sidebar** (220px fixed left) — violet/gray gradient, SVG icons
- **Two sections**: Principal (Inbox, Saved, Archive, Brief, Analytics) + Herramientas (Research, Crawler, Escritos, Watchlist, Settings)
- **Newspaper-style cards** — dark charcoal header with source name + region badge, bold titles, summary, photo
- **Responsive grid**: 1→2→3→4 columns
- **Slide-in reader panel** from right edge
- **Indigo accent**, zinc grays, minimal rounded corners
