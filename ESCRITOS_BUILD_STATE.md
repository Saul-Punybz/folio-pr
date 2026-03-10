# Escritos — SEO Article Generator Build State

## Build Order & Progress

| # | File | Status | Notes |
|---|------|--------|-------|
| 1 | `migrations/014_escritos.sql` | DONE | escritos + escrito_sources tables |
| 2 | `internal/models/escrito.go` | DONE | EscritoStore + EscritoSourceStore with full CRUD |
| 3 | `internal/generator/scrub.go` | DONE | 25 ES + 15 EN AI phrases, detect + scrub |
| 4 | `internal/generator/seo.go` | DONE | 100-point SEO scoring: 6 components |
| 5 | `internal/generator/generator.go` | DONE | 4-phase pipeline orchestrator |
| 6 | `internal/handlers/escritos.go` | DONE | 8 HTTP endpoints |
| 7 | `cmd/api/main.go` | DONE | Wire stores + handler + routes `/api/escritos` |
| 8 | `cmd/worker/main.go` | DONE | Add escritos cron (every 2 min) |
| 9 | `frontend/src/lib/api.ts` | DONE | 7 interfaces + 8 API methods |
| 10 | `frontend/src/layouts/Layout.astro` | DONE | Added "Escritos" nav link |
| 11 | `frontend/src/pages/escritos.astro` | DONE | Astro page shell |
| 12 | `frontend/src/islands/EscritosDesk.tsx` | DONE | Two-panel React island (list + detail w/ 4 tabs) |

## Compilation: PASS

```
go build ./cmd/api    -> OK
go build ./cmd/worker -> OK
```

Also fixed: removed unused `crawler` import in `internal/research/phase1.go` (pre-existing issue).

## New Files Created (8)

1. `migrations/014_escritos.sql` — 37 lines
2. `internal/models/escrito.go` — ~310 lines
3. `internal/generator/scrub.go` — ~110 lines
4. `internal/generator/seo.go` — ~290 lines
5. `internal/generator/generator.go` — ~370 lines
6. `internal/handlers/escritos.go` — ~320 lines
7. `frontend/src/pages/escritos.astro` — 8 lines
8. `frontend/src/islands/EscritosDesk.tsx` — ~680 lines

## Modified Files (4)

1. `cmd/api/main.go` — Added EscritoStore, EscritoSourceStore, EscritosHandler, 8 routes
2. `cmd/worker/main.go` — Added escritos stores + cron (*/2 * * * *)
3. `frontend/src/layouts/Layout.astro` — Added 'escritos' to nav type + nav item
4. `frontend/src/lib/api.ts` — Added 7 interfaces + 8 api methods

## Architecture Summary

### Database (014_escritos.sql)
- `escritos` — main table: topic, slug, title, meta_description, keywords[], hashtags[], content (markdown), article_plan (JSON), seo_score (JSON), status (queued|planning|generating|scoring|done|failed), publish_status (draft|reviewing|published), word_count
- `escrito_sources` — junction: escrito_id -> article_id, with relevance + used_in_section

### Model (escrito.go)
- **EscritoStore**: Create, GetByID, ListByUser, ListQueued, UpdateContent, UpdateSEOScore, UpdateStatus, UpdateProgress, SetStarted, SetFinished, SetFailed, UpdatePublishStatus, UpdateEdited, SetArticlePlan, Delete
- **EscritoSourceStore**: Create (ON CONFLICT DO NOTHING), ListByEscrito (JOIN articles), DeleteByEscrito

### Generator Package (internal/generator/)
- **scrub.go**: DetectAIPhrases() + ScrubAIPhrases() — 40 phrases ES/EN
- **seo.go**: ScoreArticle() -> SEOScore (100 pts: keyword density 20, title 15, meta 15, readability 15, structure 20, AI cleanliness 15)
- **generator.go**: 4-phase pipeline
  - Phase 1: Source gathering (fetch article IDs or search by topic, last 3 months)
  - Phase 2: Article planning (AI generates section plan JSON: intro APP + 3-5 H2 + FAQ + conclusion)
  - Phase 3: Section-by-section generation + AI metadata (title, slug, meta, keywords, hashtags) + scrub
  - Phase 4: SEO scoring

### Handler (escritos.go) — 8 endpoints
| Method | Path | Action |
|--------|------|--------|
| POST | `/api/escritos` | Create + launch background generation |
| GET | `/api/escritos` | List user's escritos |
| GET | `/api/escritos/{id}` | Get detail + sources |
| PUT | `/api/escritos/{id}` | Edit title/meta/content/keywords/hashtags/publish_status |
| POST | `/api/escritos/{id}/regenerate` | Reset + re-run |
| POST | `/api/escritos/{id}/seo-score` | Recalculate SEO |
| DELETE | `/api/escritos/{id}` | Delete (cascade) |
| POST | `/api/escritos/{id}/export` | Export as markdown or HTML |

### Frontend (EscritosDesk.tsx)
- **Left panel**: Escrito list with status badges + mini SEO score indicator + word count
- **Right panel**: Detail view with 4 tabs:
  - **Contenido**: View mode (rendered markdown + SERP preview) / Edit mode (title/meta char counters + markdown textarea)
  - **SEO**: Score gauge (0-100), 6 component breakdown bars, warnings/recommendations, Recalcular button
  - **Fuentes**: Source article list with title, source, URL, relevance badge
  - **Exportar**: Copy Markdown / Copy HTML buttons, hashtag display for social, format preview
- Auto-polls every 5s when status is active
- Publish status toggle (Borrador / En revision / Publicado)

## Testing Steps

1. Start API -> migration 014 auto-applies
2. `curl -X POST localhost:8080/api/escritos -H 'Content-Type: application/json' -d '{"topic":"energia renovable en Puerto Rico"}'`
3. Poll `GET /api/escritos/{id}` -> watch status: queued -> planning -> generating -> scoring -> done
4. Verify content has sections, SEO score populated, hashtags present
5. Edit title -> POST seo-score -> verify score updates
6. POST export -> verify markdown/HTML output
7. Frontend: open `/escritos`, create article, watch generation, edit, check SEO tab
