# Folio

App de Mac para una organización sin fines de lucro de Puerto Rico. Cada mañana
se abre Folio y ahí está, **sin IA y sin buscar**, lo que salió de sus marcas y
qué fondos abrieron.

> Hecho por [Saul A. González Alonso](https://github.com/Saul-Punybz).

## Instalar

Mac con chip de Apple (M1 o más nuevo). Abre **Terminal** y pega:

```
curl -fsSL https://github.com/Saul-Punybz/folio-pr/releases/latest/download/instalar.sh | bash
```

Instala Folio en Aplicaciones, lo deja arrancando solo al encender la Mac y abre
el navegador en `http://127.0.0.1:47615`. La misma línea actualiza; los datos se
quedan. Guía paso a paso: [docs/INSTALAR.md](docs/INSTALAR.md).

## Qué hace

**Noticias.** Medios de Puerto Rico y de fuera que hablan de Puerto Rico. El Inbox
muestra los últimos 7 días; lo que no se toca pasa al Archivo y se queda ahí.
«Borrar» lo quita de verdad y esa noticia no vuelve.

**Marcas.** Por cada marca de la organización, Folio busca menciones en noticias,
sus sitios propios, Bluesky, Mastodon, YouTube, podcasts y Alertas de Google.
Cada mención se archiva con copia del texto, resumen y cita en formato APA, y la
bibliografía se exporta.

**Fondos.** Subastas, RFP y grants:

- páginas y PDF de agencias del gobierno de Puerto Rico;
- municipios de Caguas, Bayamón y Guaynabo;
- Grants.gov, solo lo que es elegible para organizaciones sin fines de lucro.

De cada convocatoria saca por patrones la referencia, la fecha límite, el monto y
el contacto, y marca las enmiendas. La persona decide qué guardar. Lo guardado se
filtra por Grants, Subastas y RFP, y conserva su PDF.

Folio busca solo mientras está abierto: noticias y marcas cada 3 horas, fondos cada
2 horas, y al abrirlo si la Mac estuvo apagada.

## Lo que no hace

- No usa IA de ningún tipo.
- No sube datos a ningún servidor: todo vive en esa Mac y solo se abre desde ella.
- No lee municipios que publican en imágenes o JavaScript, ni el texto de PDF
  escaneados (quedan marcados con su título).

## Cómo está hecho

Un solo binario de Go con la API, el frontend (Astro + React), las migraciones y
las tareas programadas adentro. `Folio.app` trae un Postgres 18 embebido: no hay
que instalar nada más.

```
cmd/folio/        binario: servidor, modo app, tareas programadas, `folio user`
internal/
  crawler/        FolioBot: robots.txt, PDF, listados, fondos, Grants.gov
  extract/        patrones de convocatorias (fecha límite, referencia, monto, contacto)
  watch/          Marcas: búsqueda de menciones por fuente
  archivo/        copia, resumen y cita APA de cada mención
  podcasts/       episodios de Apple Podcasts
  pdftext/        texto de PDF en Go puro
  embeddb/        Postgres embebido
  grants/ robots/ scraper/ handlers/ models/ middleware/ ...
migrations/       SQL, se aplican solas al arrancar
frontend/         Astro + React, Material 3 Expressive
scripts/          build-macos.sh, instalar.sh
```

## Construir

```bash
make mac                                         # build/Folio.app y build/Folio.zip
cd frontend && npx tsc --noEmit -p . && npm run build
gofmt -l . && go build ./... && go vet ./... && go test -race ./...
make run-app                                     # la app en primer plano, sin abrir navegador
```

Hace falta Go, curl y npm. El script baja y verifica (SHA-256) los binarios de
Postgres. Para publicar una versión: `make mac` y subir `build/Folio.zip` y
`scripts/instalar.sh` a un release nuevo marcado como el último.

## Usuarios

No hay usuario por defecto. La primera vez, Folio pide crear el acceso. Por terminal:

```bash
FOLIO_PASSWORD='...' /Applications/Folio.app/Contents/MacOS/Folio user add persona@org.org admin
/Applications/Folio.app/Contents/MacOS/Folio user list
/Applications/Folio.app/Contents/MacOS/Folio user reset persona@org.org
```

## Datos

- Todo en `~/Library/Application Support/Folio2/`: base, PDF guardados, registros.
- Respaldo diario en `Folio2/respaldos/`; se guardan los últimos 14.
- Para cerrar Folio: Ajustes → «Cerrar Folio».

## Licencia

Ver [LICENSE](LICENSE).
