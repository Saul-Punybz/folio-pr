# Folio — instrucciones para trabajar en el repo

App de Mac, un solo binario de Go, **sin IA** (nada de Ollama ni servicios de IA),
sin Docker, sin S3, sin Caddy. Estado y plan: `CONTINUAR.md`; meta: `VISION.md`.

## Construir y probar

```bash
make mac                                         # build/Folio.app + build/Folio.zip
cd frontend && npx tsc --noEmit -p . && npm run build
gofmt -l . && go build ./... && go vet ./... && go test -race ./...
FOLIO_NO_BROWSER=1 build/Folio.app/Contents/MacOS/Folio app   # probar sin abrir navegador
```

Las pruebas que necesitan Postgres usan los binarios de `build/Folio.app` (o
`FOLIO_PG_BIN`) y se saltan si no están. Para pruebas manuales usa un `HOME`
temporal: la app guarda todo en `$HOME/Library/Application Support/Folio2`.
Nunca toques `Folio` ni `Folio2` de la usuaria real.

## Arquitectura

- `cmd/folio`: `main.go` (servidor), `app.go` (modo app: Postgres embebido, 127.0.0.1:47615),
  `cron.go` (tareas con candado; cadena de fondos crawl → extracción → Grants.gov → estados),
  `router.go`, `user.go` (`folio user add|reset|list`; sin `DB_*` usa el Postgres embebido).
- `internal/crawler`: FolioBot (`FolioBot/1.0`), robots.txt y ≥1 s por host, PDF a
  `<datos>/documentos/<sha256>.pdf`, listados partidos en filas (`rows.go`), huella
  de deduplicado (`dedupe.go`), extracción (`extraction.go`), Grants.gov (`grantsgov.go`).
- `internal/extract`: patrones de convocatorias; único parser de montos (`AmountCents`).
- `internal/podcasts`: episodios de Apple Podcasts para Marcas (`watch.ScanPodcasts`, fuente `podcast`).
- `internal/gdelt`: cliente listo pero **apagado** (no está en el cron ni en el escaneo): 0 resultados para las marcas y límite de 1 petición cada 5 s.
- `internal/pdftext`, `internal/robots`, `internal/grants`, `internal/embeddb`.
- Migraciones en `migrations/`, en orden por nombre, embebidas y aplicadas al arrancar.

## Variables

Ver `.env.example`. En modo app no hace falta ninguna.
