# Folio — CONTINUAR

**Este es el único documento que dice qué está construido.** `VISION.md` dice qué se quiere lograr. `CONTINUE.md`, `CONTINUE_CRAWLER.md`, `CRAWLER_BUILD_STATE.md`, `ESCRITOS_BUILD_STATE.md`, `ESCRITOS_CONTINUATION.md` y `SPEC.md` son historia de marzo 2026 y se borran en la tanda 1.

Para retomar: "CONTINUE FOLIO" → leer este archivo.

## Arranque de la obra (14 nov 2026)

### Meta

Que tres personas de la organización abran Folio en el navegador y cada mañana encuentren, sin IA y sin buscar, lo que salió de sus marcas y qué fondos abrieron.

### Decisiones tomadas

- Hospedaje: Vercel (cuenta de Saul, usuario `saul-6081`) + Supabase (proyecto `gcmoegzjnxrmtjtukkia`).
- Cero IA dentro de Folio. Ollama sale. La única IA es el botón "Copiar para Claude" en Escritos, que usa el plan de la organización fuera de Folio. MCP queda para después.
- Login: se restaura el de sesiones propio que ya existía (`internal/middleware/auth.go` `SessionAuth`, `internal/handlers/auth.go`, y `LoginForm.tsx` + `login.astro` desde git). Usuarios creados por script; nada de `admin/admin`.
- Blog: Wix por API; otros por exportación HTML/Markdown.

### Lista mínima, en el orden del camino

| # | Pieza | Estado | Tanda |
|---|---|---|---|
| 1 | Limpieza: fuera app de Mac (Wails, Postgres embebido), Ollama, Inteligencia/chat, Research, Analytics, grafo de entidades, Telegram. Un solo router. | FALTA | 1 |
| 2 | Login con sesiones, usuarios por script | FALTA | 1 |
| 3 | Ingesta RSS sin IA (resumen = descripción del medio) | FALTA (hoy depende de IA) | 1 |
| 4 | Base en Supabase con las migraciones aplicadas | FALTA — espera la cadena de conexión | 2 |
| 5 | Despliegue en Vercel: API Go + Astro + cron de ingesta | FALTA | 2 |
| 6 | Vigilancias sin IA: marcas, personas y temas por términos; resumen diario | FALTA | 3 |
| 7 | Fondos: APIs de Grants.gov y SAM.gov; crawler de PR con PDF, extracción por patrones y confirmación con un clic | FALTA | 4 |
| 8 | Páginas fijas diarias con lo que cambió | FALTA | 4 |
| 9 | Guardados con carpetas por tema y citas | FALTA | 5 |
| 10 | Escritos: editor, paquete de fuentes, lista SEO/GEO, "Copiar para Claude" | FALTA | 5 |
| 11 | Enviar por email y publicar a Wix | FALTA | 6 |

**Esta obra cubre las piezas 1 a 5.** Las demás son obras siguientes.

### Lo que NO se construye

- Servidor MCP ni llamadas a ninguna API de IA.
- App de escritorio.
- Bot de Telegram, Research, Analytics, grafo de entidades, chat.
- Archivo de evidencia en S3.
- Fuentes de EE. UU. con filtro de relevancia por IA.

### Terminado, por pieza

- **1–3:** `go build ./... && go vet ./...` limpio sin el paquete `internal/ai`; `npx tsc --noEmit` sin errores; se corre el binario local, se entra con login y el Inbox muestra noticias traídas en esa corrida.
- **4:** las 25 migraciones (más las nuevas) registradas en `_migrations` en Supabase.
- **5:** se abre la URL de Vercel, pide login, se entra, y el Inbox muestra noticias traídas por el cron, no por una corrida manual.

### Presupuesto (equivalencias en precio de API, no cargos)

| Partida | Modelo | Estimado |
|---|---|---|
| Orquestador, ~2 h | Opus | $9 |
| Tanda 1 · backend: quitar IA de ingesta y handlers, router único, login | Opus | $5 |
| Tanda 1 · frontend: login, quitar páginas, arreglar 3 errores de tipos | Sonnet | $3 |
| Tanda 2 · Vercel + cron + conexión Supabase | Sonnet | $3 |
| Verificadores de cierre (2) | Haiku | $1 |
| Reserva para segunda vuelta | — | $9 |
| **Total** | | **~$30, 2 tandas** |

### Archivos compartidos (solo los toca el orquestador)

`cmd/api/main.go` · `frontend/src/lib/api.ts` · `frontend/src/layouts/Layout.astro` · `migrations/` · `go.mod` / `go.sum` · `internal/config/config.go` · `vercel.json`

### Por confirmar al abrir la tanda 2

- Si el backend va en el runtime de Go de Vercel o como imagen de contenedor (ya hay `Dockerfile`).
- Duración máxima por función y frecuencia de cron en el plan de la cuenta.
- Modo de conexión de pgx con el pooler de Supabase.

## Estado

- 14 nov 2026 — Todo el trabajo suelto de marzo quedó en la rama `estado-marzo-2026` (commit `de08f5b`), subida a GitHub. `main` sin tocar.
- 14 nov 2026 — Revisión del código: compila, cero pruebas. Errores conocidos del crawler y de la extracción de oportunidades (se arreglan en la tanda 4, cuando se rehace Fondos): páginas marcadas como revisadas aunque falle la extracción; `parseAmountCents` multiplica por un millón con cualquier "m"; `crawler.go:129` guarda un carácter basura en vez del código HTTP; sin `robots.txt`; sin PDF.
- Datos locales de marzo (1,530 artículos, 21 oportunidades, 361 páginas) siguen en `~/Library/Application Support/Folio/pgdata`. No se migran salvo que Saul lo pida.
