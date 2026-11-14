# Folio

App de Mac para una organización sin fines de lucro de Puerto Rico. Cada mañana
se abre Folio y se ve, **sin IA**, lo que salió de sus marcas en las noticias y
los **fondos abiertos** (subastas, RFP, grants) en cuatro niveles: federal,
gobierno central (estatal), municipio y local.

> Hecho por [Saul A. González Alonso](https://github.com/Saul-Punybz).

## Qué hace

- **Noticias:** RSS y páginas de medios de PR; el resumen es la descripción del medio.
- **Vigilancias:** marcas, personas y temas por términos.
- **Fondos:** FolioBot rastrea páginas de subastas y avisos del gobierno (Registro
  Único de Subastas, ASG, agencias y municipios), baja los PDF enlazados, saca su
  texto y encuentra por patrones la referencia, la fecha límite, el monto y el
  contacto. Además consulta la API pública de Grants.gov. Todo entra como
  «posible fondo»: la persona lo confirma o lo descarta, y lo descartado no vuelve.
- **Guardados y exportación** de artículos.

Nada usa IA: ni Ollama ni servicios externos de IA. No hay Docker, S3 ni Caddy.

## Cómo está hecho

Un solo binario de Go (`cmd/folio`) con la API (Chi), el frontend (Astro + React)
y las migraciones embebidos, y las tareas programadas (cron) adentro. En la Mac,
`Folio.app` trae además un Postgres 18 embebido: no hace falta instalar nada.

```
cmd/folio/        binario: servidor, modo app, cron, `folio user`
internal/
  crawler/        FolioBot: cola, robots.txt, PDF, listados en filas, fondos, Grants.gov
  extract/        patrones de convocatorias (fecha límite, referencia, monto, contacto)
  pdftext/        texto de PDF en Go puro
  robots/         robots.txt por host
  grants/         cliente de la API de Grants.gov
  embeddb/        Postgres embebido (datos en ~/Library/Application Support/Folio2)
  handlers/ models/ middleware/ scraper/ watch/ ...
migrations/       SQL, se aplican solos al arrancar
frontend/         Astro + React
scripts/          build-macos.sh
```

## Construir

```bash
make mac          # = bash scripts/build-macos.sh → build/Folio.app y build/Folio.zip
```

Requisitos para compilar: Go, curl y npm (si falta `frontend/dist`). El script baja
y verifica (SHA-256) los binarios de Postgres.

Otras tareas:

```bash
cd frontend && npm install && npm run build   # frontend
go build ./... && go vet ./... && go test ./... # Go (las pruebas con base usan build/Folio.app)
make run-app                                    # la app en primer plano, sin abrir navegador
```

## Usuarios

No hay usuario por defecto. La primera vez, la app pide crear la cuenta de
administración. También se pueden crear por terminal (con Folio abierto o cerrado):

```bash
FOLIO_PASSWORD='...' build/Folio.app/Contents/MacOS/Folio user add persona@org.org admin
build/Folio.app/Contents/MacOS/Folio user list
```

## Datos

- Base, respaldos diarios y registros: `~/Library/Application Support/Folio2/`.
- PDF de los fondos: `…/Folio2/documentos/<sha256>.pdf`.
- Fuera del modo app (`folio serve` con `DB_*`), los PDF van a `FOLIO_DATA_DIR` (por defecto `./data`).

Para cerrar Folio: Ajustes → «Cerrar Folio».

## Licencia

Ver [LICENSE](LICENSE).
