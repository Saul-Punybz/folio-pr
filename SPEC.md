# Folio
## Political Intelligence & Evidence Archive System
Go + Astro + Oracle Free Tier + Self-Hosted AI

---

## Architecture Decisions (Locked)

| Area | Decision |
|---|---|
| Go framework | Chi |
| Auth | Session cookies + bcrypt + Postgres |
| Domain | DuckDNS (free) + Caddy auto-HTTPS |
| Frontend | Static Astro + Tailwind + React Islands, served by Go |
| Deployment | Docker Compose on Oracle Free Tier ARM |
| DB | Postgres + pgvector, pgx driver |
| AI | llama3.2:3b (instruct) + nomic-embed-text (embeddings) |
| Object storage | Oracle S3-compatible via aws-sdk-go-v2 |
| Ingestion | RSS first, Colly HTML scraper fallback |
| Daily cap | 100 articles (85 PR / 15 Grants) |
| Regions | PR + Grants (DC deferred) |
| Sources | 25 configured (5 RSS, 20 HTML) |
| Scrape schedule | 6 runs/day via robfig/cron |

---

## Vision

Folio is a lightweight political intelligence system focused on:
- Puerto Rico news
- Grants / NOFO / Funding announcements

Designed for:
- Maximum 5 team members
- 100 articles per day
- Simple interface — kid-friendly triage
- Long-term searchable archive
- Evidence retention (3 / 6 / 12 months / Keep forever)

---

## Core Principles

1. Simple decisions: Save or Trash
2. Everything else automated
3. Evidence preserved, but expirable
4. Fast navigation
5. Low infrastructure cost
6. Free-tier compatible

---

## System Architecture

### Frontend
- Astro (static output)
- TailwindCSS
- React Islands (only where needed)
- Dark / Light mode toggle

### Backend
- Go (Chi router)
- REST JSON API
- Colly (scraper)
- Cron jobs (6 runs per day)
- pgvector for similarity

### Database
- Postgres (self-hosted on Oracle VM) + pgvector

### AI (Self-hosted)
- Ollama
- llama3.2:3b (instruct — summaries, classification, entity extraction)
- nomic-embed-text (embeddings)

### Storage
- Oracle Object Storage (S3-compatible)
- Evidence lifecycle by prefix:
  - evidence/ret-3m/
  - evidence/ret-6m/
  - evidence/ret-12m/
  - evidence/keep/

---

## Ingestion Priority (per source)

1. RSS/Atom feed → parse XML
2. Sitemap.xml → extract URLs, fetch + extract
3. Category page scraping → Colly with selectors
4. Manual collect → POST /api/collect with a URL

---

## Core Features

### Inbox (Triage Mode)
- Save / Trash / Pin buttons
- Keyboard shortcuts: S=Save, T=Trash, P=Pin, J/K=Navigate
- Optimistic UI, undo toast (10 seconds)

### Saved Feed
- All saved + pinned items
- Auto tags, notes & comments
- Retention badge, quick search

### Archive
- Full-text search across all time
- Filters: date range, region, source, status
- "Evidence expired" badge after expiration

### Evidence Archive
- raw.html.gz, extracted.json.gz, capture_meta.json, SHA-256 hash
- Retention: 3m / 6m / 12m / Keep forever
- Trash default: 3 months, block URL re-ingestion

---

## Database Schema

### articles
id, title, source, source_id, url, canonical_url, region, published_at, clean_text, summary, status, pinned, evidence_policy, evidence_expires_at, tags, embedding, created_at

### fingerprints
id, canonical_url_hash, content_hash, blocked, created_at

### users
id, email, password_hash, role, created_at

### sessions
id, user_id, expires_at, created_at

### notes
id, article_id, user_id, content, created_at

### sources
id, name, base_url, region, feed_type, feed_url, list_urls, link_selector, title_selector, body_selector, date_selector, active, created_at

---

## API Endpoints

Auth:
- POST /api/login
- POST /api/logout
- GET /api/me

Inbox:
- GET /api/items?status=inbox
- POST /api/items/{id}/save
- POST /api/items/{id}/trash
- POST /api/items/{id}/pin

Search:
- GET /api/search?q=&from=&to=&region=&status=

Collect:
- POST /api/collect { url }

Admin:
- GET /api/sources
- POST /api/sources
- PUT /api/sources/{id}

Health:
- GET /api/health

---

## Sources (25)

### PR — RSS
1. El Nuevo Dia (RSS)

### PR — Scrape
2. Primera Hora
3. El Vocero
4. NotiCel
5. Metro Puerto Rico
6. Noticentro (WAPA)
7. WAPA.TV
8. Telemundo Puerto Rico
9. Univision Puerto Rico
10. TeleOnce
11. Radio Isla 1320
12. CPI
13. News is My Business
14. Caribbean Business
15. El Calce
16. Es Noticia PR
17. Jay Fonseca
18. Senado de Puerto Rico
19. Camara de Representantes
20. La Fortaleza

### Grants — RSS
21. Grants.gov
22. GAO Reports
23. GovInfo
24. Federal Register

---

## Milestone Plan

### Phase 1 (Current)
- Auth, Inbox UI, Save/Trash, Basic scraping, Evidence storage, Retention lifecycle

### Phase 2
- Full-text search, Embeddings + clustering, Summaries, Archive filters

### Phase 3
- Notes, Daily brief, Alerts, Export (PDF / ZIP)

---

## Success Criteria

- Kid can triage 30 items in < 5 minutes
- Team can search 6 months back instantly
- Evidence auto-expires correctly
- No manual scraping maintenance daily
- Runs fully on Oracle Free Tier
