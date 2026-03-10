# Folio — Continuar en Nuevo Terminal

## Instrucciones para Claude Code

Abre un nuevo terminal en `/Users/saulgonzalez/Downloads/folio/` y ejecuta:

```
claude
```

Luego pega esto:

---

**CONTINUE FOLIO — Crawler + Escritos Testing**

Lee estos archivos para contexto:
1. `CRAWLER_BUILD_STATE.md` — lo que se hizo con el Crawler (Opción C completa)
2. `ESCRITOS_BUILD_STATE.md` — lo que se hizo con Escritos (SEO Generator)
3. `CONTINUE.md` — guía general del proyecto

## Lo que se acaba de completar (esta sesión):

### Crawler — Opción C (Full UI + Integration)
- `frontend/src/islands/CrawlerDesk.tsx` (1,361 líneas) — 5 tabs: Dashboard, Dominios, Páginas, Cambios, Grafo
- `frontend/src/pages/crawler.astro` — page shell
- `frontend/src/layouts/Layout.astro` — nav link added
- `frontend/src/lib/api.ts` — 7 interfaces + 16 API methods
- `internal/research/phase1.go` — 7th search agent (crawl_index) conectado
- `cmd/worker/main.go` — CrawledPages wired to research Deps
- `internal/handlers/crawler.go` — TriggerCrawl context bug fixed
- **Go backend compila limpio**: `go build ./cmd/api && go build ./cmd/worker`

### Escritos — SEO Article Generator (sesión anterior)
- 8 nuevos archivos + 4 modificados
- Pipeline de 4 fases: source gathering → planning → generation → SEO scoring
- Frontend: EscritosDesk.tsx con 4 tabs
- **Compila pero sin testear end-to-end**

## Lo que falta hacer (en orden de prioridad):

1. **Build frontend**: `cd frontend && npm run build && cd ..`
2. **Iniciar servicios**: PostgreSQL, Ollama, API, Worker
3. **Test Crawler UI**: Navegar a `/crawler`, verificar Dashboard, Dominios, Páginas
4. **Test Escritos**: Crear un escrito, verificar generación con Ollama
5. **Test Research + Crawler**: Crear research project, verificar que Phase 1 incluye crawl_index
6. **Test Telegram bot**: Verificar comandos existentes siguen funcionando
7. **Considerar**: Agregar comandos Telegram para crawler (/crawl status, /crawl trigger)

## Comandos útiles:

```bash
# Build
go build -o bin/api ./cmd/api && go build -o bin/worker ./cmd/worker && cd frontend && npm run build && cd ..

# Run
./bin/api          # API + frontend en :8080
./bin/worker       # Cron jobs
./bin/bot          # Telegram bot

# DB
/opt/homebrew/Cellar/postgresql@17/17.9/bin/psql -U folio -d folio

# Login
# admin@folio.local / admin
```

## Auditoría Completa de Folio (contexto):

Se hizo una auditoría de TODAS las partes de Folio. Resultado:
- 12 features en total, 11 completamente conectados end-to-end
- Crawler era el único desconectado — ahora está reparado con UI completa
- Escritos recién creado, necesita testing
- Todo lo demás (Inbox, Saved, Archive, Brief, Watchlist, Research, Analytics, Settings, Telegram, Scraper) funciona correctamente

---
