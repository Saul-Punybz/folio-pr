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

### Latest Session — Chat Sessions + Saved Articles Fix (Feb 28)

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
- **RSS ingestion**: El Nuevo Dia, GAO Reports, CBO Reports (RSS-based sources)
- **AI enrichment**: Ollama generates summaries, tags, embeddings for each article
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
1. **Configure scrape-based sources** -- 19 sources need `list_urls` and CSS selectors
2. **Fix RSS feeds that fail parsing**:
   - Federal Register (Atom format not supported)
   - GovInfo (feed URL format issue)
   - Grants.gov (XML format issue)

#### Medium Priority
3. **Deploy to Oracle VM** -- `scripts/deploy-oracle.sh` exists but untested
4. **Evidence retention lifecycle** -- cron cleanup runs but needs real S3 for archive
5. **User management** -- Currently single admin user, add user creation UI
6. **Alert matching UI** -- Show matched articles inline with alert keywords highlighted
7. **Export improvements** -- Bulk export with date range selection

#### Polish / Nice-to-Have
8. **Pagination** -- Inbox loads all articles at once; add infinite scroll or pagination
9. **Dark mode refinement** -- Some newspaper-style elements need dark mode tuning
10. **Mobile responsive** -- Grid works on mobile (1 col) but action buttons could be improved
11. **Article deduplication UI** -- Show when duplicates are blocked
12. **Source health dashboard** -- Show which sources are working/failing
13. **Reading time estimate** -- Calculate from clean_text word count
14. **Keyboard shortcut help overlay** -- Show available shortcuts (? key)

## Architecture

```
cmd/api/main.go          -- HTTP server (Chi router, session auth, serves frontend)
cmd/worker/main.go       -- Background worker (cron: ingestion, cleanup, briefs)
internal/
  ai/ollama.go           -- Ollama client (summarize, embed, generate)
  config/                -- Environment config loader
  db/                    -- PostgreSQL connection (pgx), auto-migrations
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
migrations/             -- 9 SQL migrations
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
