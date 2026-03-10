# Folio -- Continuation Guide

## Quick Resume

Say **"CONTINUE FOLIO"** to pick up where we left off.

## How to Run Locally

```bash
cd /Users/saulgonzalez/Downloads/folio

# 1. Make sure PostgreSQL is running
brew services start postgresql@17

# 2. Make sure Ollama is running with required models
ollama serve &
# Models needed: llama3.2:3b, nomic-embed-text

# 3. Build backend + frontend
go build -o bin/api ./cmd/api && go build -o bin/worker ./cmd/worker
cd frontend && npm run build && cd ..

# 4. Start API (serves frontend + REST API on :8080)
./bin/api
# Or with explicit env vars:
DB_HOST=localhost DB_PORT=5432 DB_USER=folio DB_PASS=folio_dev \
DB_NAME=folio DB_SSLMODE=disable SERVER_PORT=:8080 \
OLLAMA_HOST=http://localhost:11434 OLLAMA_INSTRUCT_MODEL=llama3.2:3b \
OLLAMA_EMBED_MODEL=nomic-embed-text DOMAIN=localhost ./bin/api

# 5. Open http://localhost:8080
# Login: admin@folio.local / admin
```

## Current State (Feb 28, 2026)

### Latest Session — Deep Research / Investigacion Profunda (Feb 28, night)

**New feature: Deep Research** — Give a topic like "reciclaje" and the system sends multiple agents to investigate across the internet, collects everything (news, web pages, references), runs in the background, then presents an organized research dossier. Always in the context of Puerto Rico.

**Migration 012:** `research_projects` + `research_findings` tables with dedup indexes

**New files (11):**
- `migrations/012_research.sql` — DB schema
- `internal/models/research.go` — ResearchProjectStore + ResearchFindingStore
- `internal/research/research.go` — RunProject orchestrator
- `internal/research/queries.go` — AI keyword expansion + query generation (8-15 variations)
- `internal/research/phase1.go` — Broad search (6 agents: Google News, Bing, DDG, local DB, YouTube, Reddit)
- `internal/research/phase2.go` — Deep scrape (top 50 pages + 1-level deep crawl for top 10)
- `internal/research/phase3.go` — AI synthesis (relevance scoring, entity aggregation, timeline, dossier generation)
- `internal/handlers/research.go` — 7 HTTP endpoints (CRUD + findings + stop + run)
- `internal/telegram/research.go` — /research command (create + status)
- `frontend/src/pages/research.astro` — Astro page
- `frontend/src/islands/ResearchDesk.tsx` — Two-panel React island with 4 tabs (Hallazgos, Dossier, Entidades, Cronologia)

**Modified files (6):**
- `cmd/api/main.go` — Research stores + handler + routes
- `cmd/worker/main.go` — Research cron (every 2 min picks up queued projects)
- `cmd/bot/main.go` — ResearchProjects store in BotDeps
- `internal/telegram/bot.go` — ResearchProjects field + /research handler registration
- `internal/agents/filter.go` — Exported `IsSpamHit()` wrapper for use by research package
- `frontend/src/layouts/Layout.astro` — Added "Investigacion" nav link
- `frontend/src/lib/api.ts` — Research interfaces + 7 API methods

**API Endpoints:**
- `POST /api/research` — Create project (topic + optional keywords)
- `GET /api/research` — List user's projects
- `GET /api/research/{id}` — Get project + stats
- `GET /api/research/{id}/findings` — Paginated findings (filter: source_type)
- `POST /api/research/{id}/stop` — Cancel ongoing research
- `POST /api/research/{id}/run` — Trigger research manually
- `DELETE /api/research/{id}` — Delete project + findings

**Telegram:** `/research <tema>` creates project, `/research status` lists all

**Frontend:** `/research` page with two-panel layout, auto-polls every 10s for active projects

**Next:** Test end-to-end with running Ollama, test Telegram /research command

### Previous Session — Sources + Intelligence + Telegram Bot (Feb 28, evening)

**Phase A: Sources Fixed**
- Migration 010: Switched 6 sources from scrape to RSS (NotiCel, Radio Isla, News is My Business, Es Noticia PR, GovInfo, Federal Register)
- Configured 9 scrape sources with CSS selectors (Primera Hora, El Vocero, Metro PR, WAPA.TV, El Calce, Senado, Camara, La Fortaleza)
- Disabled 4 broken sources (Jay Fonseca, Noticentro, Caribbean Business, Grants.gov)
- Added "Test Scrape" button in SourcesManager UI — tests RSS feeds or scrape selectors inline
- New endpoint: `POST /api/sources/{id}/test`

**Phase B: Entity Graph + Trend Tracking**
- Migration 010: `entities` table, `article_entities` junction, `articles.entities` JSONB column, `articles.sentiment` column
- `internal/models/entity.go` — EntityStore (Upsert, LinkToArticle, TopEntities, CoOccurrences)
- `internal/ai/ollama.go` — Structured `ExtractEntities` (returns people/organizations/places), new `ClassifySentiment` method
- `internal/scraper/ingest.go` — Enrichment pipeline now persists entities + sentiment
- `internal/handlers/analytics.go` — 6 endpoints: tag trends, top entities, co-occurrences, sentiment, source health, volume
- `frontend/src/pages/analytics.astro` + `AnalyticsDashboard.tsx` — Full analytics dashboard with charts, entity leaderboard, sentiment breakdown, source health table
- Navigation updated with "Analytics" link

**Phase C: Telegram Bot**
- Migration 011: `telegram_users`, `bot_notifications` tables
- `internal/intelligence/` — Extracted chat logic from admin handler into reusable package
- `internal/handlers/admin.go` — Refactored ChatWithNews to delegate to intelligence.Chat()
- `internal/models/telegram_user.go` — TelegramUserStore
- `internal/models/notification.go` — NotificationStore (CreateDigest, CreateWatchlistHit, ListPending, MarkDelivered)
- `internal/telegram/` — 7 files: bot.go, auth.go, commands.go, handlers.go, callbacks.go, format.go, notify.go
- `cmd/bot/main.go` — Third binary entry point
- Bot commands: /start, /help, /inbox, /saved, /search, /brief, /watchlist + plain text AI chat
- Inline keyboards for article actions (save, trash, pin)
- Notification poller delivers digest + watchlist hit alerts
- Typing indicator for slow AI responses
- Worker emits digest notifications after daily brief generation
- Config: `TELEGRAM_BOT_TOKEN` + `TELEGRAM_ALLOWLIST` env vars

**Build:**
```bash
go build -o bin/api ./cmd/api
go build -o bin/worker ./cmd/worker
go build -o bin/bot ./cmd/bot
cd frontend && npm run build && cd ..
```

**Run Telegram bot:**
```bash
TELEGRAM_BOT_TOKEN="your-token" TELEGRAM_ALLOWLIST="telegram_id:admin@folio.local" ./bin/bot
```

### Previous Session — Chat Sessions + Saved Articles Fix (Feb 28, morning)

**1. AI Chat Session Persistence (new feature)**
- New `chat_sessions` table (migration 009) stores full conversation as JSONB
- Backend: `ChatSessionStore` model + `ChatHandler` with 5 REST endpoints
- Frontend: Historial sidebar in Inteligencia page
- Auto-save after each AI response
- Load, delete, and create new sessions from sidebar
- Files: `migrations/009_chat_sessions.sql`, `internal/models/chat_session.go`, `internal/handlers/chat.go`, `cmd/api/main.go`, `frontend/src/lib/api.ts`, `frontend/src/islands/DailyBrief.tsx`

**2. Saved Articles — Placeholder Images**
- Articles without `image_url` now show a newspaper icon placeholder (gradient background)
- Both grid cards and expanded panel have the placeholder
- Broken images also fall back to the same placeholder
- File: `frontend/src/islands/SavedFeed.tsx`

**3. Saved Articles — Snippet as Summary**
- When saving web sources from AI chat, the web search snippet is now passed as the initial summary
- `collectItem` endpoint accepts optional `snippet` field, stored as `summary`
- Even if background enrichment fails (site blocks scraping, JS-rendered content), article has useful text
- Background enrichment still runs and overwrites with AI summary if successful
- Files: `internal/handlers/items.go`, `frontend/src/lib/api.ts`, `frontend/src/islands/DailyBrief.tsx`

### Previous Session — UI Overhaul (Feb 27)

- Slide-in side panel for InboxFeed + SavedFeed
- Tag/Source/Region filter bar for InboxFeed
- Collect URL feature ("+" button)
- Source form improvements (visual tabs for feed type)
- CSS animations (slide-in, fade-in)

### What Works
- **3 Phases complete**: Inbox triage, search, embeddings, notes, briefs, alerts, export
- **AI Chat (Inteligencia)**: Ask questions about PR news, auto-saves sessions
- **Chat History**: Sessions persist across page refreshes, loadable from sidebar
- **RSS ingestion**: 10+ RSS sources (El Nuevo Dia, NotiCel, Radio Isla, GAO, CBO, GovInfo, Federal Register, etc.)
- **Scrape ingestion**: 8 scrape sources configured (Primera Hora, El Vocero, Metro PR, WAPA, etc.)
- **AI enrichment**: Ollama generates summaries, tags, entities (people/orgs/places), sentiment, embeddings
- **Entity Graph**: Deduplicated entity registry with co-occurrence tracking
- **Analytics Dashboard**: Tag trends, entity leaderboard, sentiment breakdown, source health, article volume
- **Telegram Bot**: `/inbox`, `/saved`, `/search`, `/brief`, `/watchlist`, AI chat, inline keyboards, notifications
- **Test Scrape**: Button in Settings to test RSS feeds and scrape selectors before committing
- **Images**: Extracted from RSS enclosures, media:content, og:image
- **Placeholder images**: Articles without images show newspaper icon
- **Save from AI chat**: Web sources saved with snippet as summary
- **Grid layout**: Inbox shows 4 newspaper-style cards per row (responsive)
- **Slide-in reader**: Click any card to open right-side panel
- **Inbox filters**: Tag, source, region dropdowns + text search
- **Collect URL**: "+" button to manually add articles by URL
- **Search**: Inbox has search + filters, Archive has full search
- **Daily Brief**: AI-generated summary + manual "Generate Brief Now" button
- **Keyboard shortcuts**: J/K navigate, S save, T trash, P pin, Enter expand, ESC close

### What Needs Work

#### High Priority
1. **Test Telegram bot** -- Set up BotFather token, test all commands, verify notification delivery
2. **Re-enrich existing articles** -- Run re-enrichment to extract entities + sentiment from existing ~657 articles
3. **Deploy to Oracle VM** -- `scripts/deploy-oracle.sh` exists but untested

#### Medium Priority
4. **Evidence retention lifecycle** -- cron cleanup runs but needs real S3 for archive
5. **User management** -- Currently single admin user, add user creation UI
6. **Alert matching UI** -- Show matched articles inline with alert keywords highlighted
7. **Export improvements** -- Bulk export with date range selection
8. **Watchlist hit notifications** -- Wire watchlist scan to emit notifications (worker-side)

#### Polish / Nice-to-Have
9. **Pagination** -- Inbox loads all articles at once; add infinite scroll or pagination
10. **Dark mode refinement** -- Some newspaper-style elements need dark mode tuning
11. **Mobile responsive** -- Grid works on mobile (1 col) but action buttons could be improved
12. **Article deduplication UI** -- Show when duplicates are blocked
13. **Reading time estimate** -- Calculate from clean_text word count
14. **Keyboard shortcut help overlay** -- Show available shortcuts (? key)

## Architecture

```
cmd/api/main.go          -- HTTP server (Chi router, session auth, serves frontend)
cmd/worker/main.go       -- Background worker (cron: ingestion, cleanup, briefs, watchlist)
cmd/bot/main.go          -- Telegram bot binary
internal/
  ai/ollama.go           -- Ollama client (summarize, embed, generate, entities, sentiment)
  config/                -- Environment config loader (DB, S3, Ollama, Telegram)
  db/                    -- PostgreSQL connection (pgx), auto-migrations
  intelligence/          -- Reusable chat logic (extracted from admin handler)
  telegram/              -- Telegram bot (commands, callbacks, notifications)
  handlers/              -- HTTP handlers (items, search, notes, briefs, watchlist, chat, export, sources, admin)
  middleware/            -- Auth middleware, admin check
  models/               -- DB models (article, source, user, session, note, brief, watchlist, chat_session, fingerprint)
  scraper/              -- RSS parser, Colly HTML scraper, ingestion pipeline, brief generator
  storage/              -- S3 client for evidence archival
frontend/
  src/islands/          -- React interactive components (InboxFeed, SavedFeed, DailyBrief, ArchiveSearch, etc.)
  src/lib/api.ts        -- Frontend API client
  src/lib/utils.ts      -- Helpers (timeAgo, formatDate, regionColor, etc.)
  src/layouts/          -- Astro layout with sidebar nav
  src/pages/            -- 8 pages (inbox, saved, archive, brief, watchlist, settings, login, index)
migrations/             -- 11 SQL migrations
```

## Key Files to Edit

| What | File |
|------|------|
| Inbox page + grid + filters | `frontend/src/islands/InboxFeed.tsx` |
| Saved articles | `frontend/src/islands/SavedFeed.tsx` |
| AI Chat (Inteligencia) | `frontend/src/islands/DailyBrief.tsx` |
| Archive search | `frontend/src/islands/ArchiveSearch.tsx` |
| Source management | `frontend/src/islands/SourcesManager.tsx` |
| Watchlist manager | `frontend/src/islands/WatchlistManager.tsx` |
| API endpoints (frontend) | `frontend/src/lib/api.ts` |
| CSS animations | `frontend/src/styles/global.css` |
| Sidebar nav | `frontend/src/layouts/Layout.astro` |
| Article model (Go) | `internal/models/article.go` |
| Chat session model (Go) | `internal/models/chat_session.go` |
| Chat handler (Go) | `internal/handlers/chat.go` |
| Items handler (Go) | `internal/handlers/items.go` |
| Admin handler (Go) | `internal/handlers/admin.go` |
| API routes | `cmd/api/main.go` |
| Worker cron jobs | `cmd/worker/main.go` |
| Sources seed data | `migrations/002_seed_sources.sql` |

## Database

```bash
# psql path
/opt/homebrew/Cellar/postgresql@17/17.9/bin/psql -U folio -d folio

# Useful queries
SELECT count(*), count(image_url), count(summary) FROM articles;
SELECT source, count(*) FROM articles GROUP BY source ORDER BY count DESC;
SELECT name, feed_type, active FROM sources ORDER BY name;
SELECT id, LEFT(title,50), updated_at FROM chat_sessions ORDER BY updated_at DESC;
```

## Design

The UI uses a **newspaper-inspired aesthetic**:
- Dark charcoal header bar on each card with source name + region badge
- Bold uppercase titles
- Summary text below
- Photo at the bottom of each card (or newspaper icon placeholder)
- Responsive grid: 1 col mobile, 2 tablet, 3 laptop, 4 desktop
- Click card to open slide-in reader panel from right edge
- Indigo accent color, zinc grays, minimal rounded corners (rounded-sm)
