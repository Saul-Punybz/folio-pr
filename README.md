# Folio

**Political Intelligence & Evidence Archive for Puerto Rico**

A self-hosted, AI-powered platform for monitoring, analyzing, and archiving news from Puerto Rico and federal sources. Built for journalists, researchers, NGOs, and anyone who needs to track political developments, government grants, and public accountability.

> **Made by [Saul Gonzalez Alonso](https://github.com/Saul-Punybz)**

---

## What It Does

Folio continuously ingests news from 25+ sources across Puerto Rico, classifies them with AI, and provides tools to research, save, and export evidence.

- **Automated News Ingestion** -- RSS feeds and HTML scraping from Puerto Rico media outlets, federal agencies, and grant databases
- **AI-Powered Analysis** -- Local LLM (Ollama) generates summaries, tags, and vector embeddings for every article
- **Smart Search** -- Full-text search + semantic similarity via pgvector embeddings
- **AI Chat (Inteligencia)** -- Ask questions about collected news in natural language; combines local archive with live web search
- **Chat History** -- AI conversations persist across sessions for future reference
- **Watchlist Monitoring** -- Track organizations with multi-engine web search (DuckDuckGo + Bing) and sentiment analysis
- **Evidence Archive** -- Save, pin, annotate, and export articles as ZIP evidence packages
- **Daily Briefs** -- AI-generated news summaries delivered automatically
- **RSS Feed Output** -- Generate personal RSS feeds from watchlist alerts

## Screenshots

| Inbox | AI Chat | Saved |
|-------|---------|-------|
| Newspaper-style grid with filtering | Ask questions, save web sources | Evidence archive with reader panel |

## Tech Stack

| Layer | Technology |
|-------|-----------|
| **Backend** | Go 1.22+ with [Chi](https://github.com/go-chi/chi) router |
| **Frontend** | [Astro 4](https://astro.build) + React Islands + Tailwind CSS |
| **Database** | PostgreSQL 17 + [pgvector](https://github.com/pgvector/pgvector) |
| **AI** | [Ollama](https://ollama.ai) -- llama3.2:3b (analysis) + nomic-embed-text (embeddings) |
| **Scraping** | [Colly](https://github.com/gocolly/colly) + Go RSS parser |
| **Deployment** | Docker Compose, designed for Oracle Free Tier ARM |
| **Reverse Proxy** | Caddy (auto-HTTPS) |
| **Object Storage** | S3-compatible (Oracle Object Storage) |

## Architecture

```
                    +------------------+
                    |   Caddy (HTTPS)  |
                    +--------+---------+
                             |
                    +--------v---------+
                    |   Go API Server  |----> PostgreSQL + pgvector
                    |   (Chi Router)   |----> Ollama (AI)
                    +--------+---------+----> S3 Storage
                             |
              +--------------+--------------+
              |                             |
     +--------v--------+          +--------v--------+
     | Astro Frontend   |          |   Go Worker     |
     | React Islands    |          | RSS Ingestion   |
     | Tailwind CSS     |          | AI Enrichment   |
     +------------------+          | Brief Generator |
                                   +-----------------+
```

### Project Structure

```
cmd/
  api/           -- HTTP server entry point
  worker/        -- Background worker (cron jobs)
internal/
  ai/            -- Ollama client (summarize, classify, embed, chat)
  agents/        -- Watchlist scanning agents (Google, Bing, web, Reddit, YouTube)
  config/        -- Environment configuration
  db/            -- PostgreSQL connection + auto-migrations
  handlers/      -- HTTP handlers (items, search, chat, watchlist, briefs, export, admin)
  middleware/    -- Session auth, admin check
  models/        -- Database models + queries
  scraper/       -- RSS parser, HTML scraper, ingestion pipeline
  storage/       -- S3 object storage client
frontend/
  src/islands/   -- React interactive components
  src/lib/       -- API client, utilities
  src/layouts/   -- Astro layouts (sidebar navigation)
  src/pages/     -- 8 pages (inbox, saved, archive, brief, watchlist, settings, login)
migrations/      -- 9 SQL migration files (auto-applied on startup)
scripts/         -- Setup, deployment, and Ollama initialization
configs/         -- Caddyfile for reverse proxy
```

## Features

### News Monitoring
- 25+ configured sources (El Nuevo Dia, Primera Hora, Metro PR, NotiCel, Radio Isla, News is My Business, GAO, CBO, Federal Register, Grants.gov, and more)
- RSS + HTML scraping with multiple selector strategies
- Automatic deduplication via URL fingerprinting
- 6 ingestion runs per day

### AI Enrichment
- Every article gets an AI-generated summary (Spanish)
- Automatic tag classification (politics, education, health, infrastructure, etc.)
- Vector embeddings for semantic similarity search
- Garbage detection clears low-quality AI outputs

### Inbox Triage
- Newspaper-style 4-column responsive grid
- Filter by tag, source, region, or search text
- Slide-in reader panel (no page navigation)
- Keyboard shortcuts: J/K navigate, S save, T trash, P pin, Enter expand, ESC close
- Collect articles by URL from any site

### AI Chat (Inteligencia)
- Natural language questions about Puerto Rico news
- Combines local article archive + live multi-engine web search
- Shows local sources and web sources with direct links
- Save web sources directly to your archive with one click
- Persistent chat sessions with history sidebar
- Quick question buttons for common topics

### Watchlist
- Monitor organizations, politicians, or topics
- Multi-engine search (DuckDuckGo + Bing News)
- Sentiment analysis (positive/neutral/negative)
- AI-drafted reports for each alert
- Personal RSS feed for watchlist alerts
- Unseen count badge

### Evidence Management
- Save articles with retention policies (3m, 6m, 12m, keep forever)
- Pin important articles
- Add notes/annotations to any article
- Export as ZIP evidence packages
- Archive search with multiple filters

### Daily Briefs
- AI-generated news summaries from recent articles
- Manual "Generate Brief Now" button
- Historical brief archive

## Getting Started

### Prerequisites

- Go 1.22+
- Node.js 18+
- PostgreSQL 17 with pgvector extension
- Ollama with `llama3.2:3b` and `nomic-embed-text` models

### Quick Start

```bash
# Clone
git clone https://github.com/Saul-Punybz/folio-pr.git
cd folio-pr

# Set up environment
cp .env.example .env
# Edit .env with your database credentials

# Install Ollama models
ollama pull llama3.2:3b
ollama pull nomic-embed-text

# Build
go build -o bin/api ./cmd/api
cd frontend && npm install && npm run build && cd ..

# Run (migrations apply automatically)
./bin/api

# Open http://localhost:8080
# Default login: admin@folio.local / admin
```

### Docker Compose

```bash
docker compose up -d --build
bash scripts/setup-ollama.sh
```

### Environment Variables

See [`.env.example`](.env.example) for all configuration options:

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_HOST` | PostgreSQL host | `localhost` |
| `DB_PORT` | PostgreSQL port | `5432` |
| `DB_USER` | Database user | `folio` |
| `DB_PASS` | Database password | `folio_dev` |
| `DB_NAME` | Database name | `folio` |
| `SERVER_PORT` | API server port | `:8080` |
| `OLLAMA_HOST` | Ollama API URL | `http://localhost:11434` |
| `OLLAMA_INSTRUCT_MODEL` | LLM for summaries/chat | `llama3.2:3b` |
| `OLLAMA_EMBED_MODEL` | Embedding model | `nomic-embed-text` |

## API Endpoints

### Public
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/health` | Health check |
| `POST` | `/api/login` | Authenticate |
| `GET` | `/feed/{token}.xml` | Watchlist RSS feed |

### Authenticated
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/items` | List articles by status |
| `POST` | `/api/items/{id}/save` | Save article |
| `POST` | `/api/items/{id}/trash` | Trash article |
| `POST` | `/api/items/{id}/pin` | Toggle pin |
| `POST` | `/api/collect` | Collect article by URL |
| `GET` | `/api/search` | Full-text search |
| `GET` | `/api/items/{id}/similar` | Semantic similarity |
| `GET/POST` | `/api/items/{id}/notes` | Article notes |
| `GET/POST` | `/api/briefs/*` | Daily briefs |
| `GET/POST/PUT/DELETE` | `/api/chat/sessions/*` | Chat sessions |
| `GET/POST/PUT/DELETE` | `/api/watchlist/*` | Watchlist management |
| `GET` | `/api/items/{id}/export` | Export as ZIP |

### Admin
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/admin/chat` | AI chat with news |
| `POST` | `/api/admin/ingest` | Trigger ingestion |
| `POST` | `/api/admin/reenrich` | Re-enrich articles |

## Database

9 migrations auto-applied on startup:

1. `001_init.sql` -- Core tables (articles, users, sessions, sources)
2. `002_seed_sources.sql` -- Pre-configured PR news sources
3. `003_phase2_enhancements.sql` -- Notes, search indexes, embeddings
4. `004_phase3_briefs_alerts.sql` -- Daily briefs, alerts system
5. `005_add_image_url.sql` -- Article images
6. `006_fix_regions_retag.sql` -- Region corrections
7. `007_watchlist.sql` -- Watchlist organizations + hits
8. `008_feed_token.sql` -- RSS feed tokens
9. `009_chat_sessions.sql` -- AI chat session persistence

## Deployment

Designed for **Oracle Cloud Free Tier** (ARM, 24GB RAM):

```bash
bash scripts/deploy-oracle.sh
```

Uses Docker Compose with Caddy for automatic HTTPS via DuckDNS.

## News Sources

### Puerto Rico Media
El Nuevo Dia, Primera Hora, Metro PR, NotiCel, Radio Isla, Caribbean Business, El Vocero, Es Noticia PR, Noticentro, TeleOnce, Telemundo PR, Univision PR, WAPA TV, News is My Business, CPI (Centro de Periodismo Investigativo), El Calce, Jay Fonseca

### Government
La Fortaleza, Senado de PR, Camara de Representantes

### Federal
GAO Reports, CBO Reports, Federal Register, GovInfo, Grants.gov

## Contributing

Contributions are welcome. Please open an issue first to discuss what you would like to change.

## License

See [LICENSE](LICENSE) for details.

---

**Made by [Saul Gonzalez Alonso](https://github.com/Saul-Punybz)**
