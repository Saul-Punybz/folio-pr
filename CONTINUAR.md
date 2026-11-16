# Folio — CONTINUAR

**Este es el único documento que dice qué está construido.** `VISION.md` dice qué se quiere lograr. Los documentos de estado de marzo 2026 (`CONTINUE.md`, `SPEC.md` y otros cuatro) se borraron en la tanda 1; siguen en la rama `estado-marzo-2026`.

Para retomar: "CONTINUE FOLIO" → leer este archivo.

## Arranque de la obra (14 nov 2026)

### Meta

Que tres personas de la organización abran Folio en el navegador y cada mañana encuentren, sin IA y sin buscar, lo que salió de sus marcas y qué fondos abrieron.

### Decisiones tomadas

- Hospedaje: **servidor propio en Oracle Cloud (Always Free), región Ashburn.** Decidido el 14 nov 2026 en lugar de Vercel + Supabase: costo $0 con respaldos, Go corre tal cual, sin límite de tiempo para el crawl y con sitio para OCR. Un solo binario (API + cron + frontend y migraciones embebidos) con Postgres y Caddy instalados en el servidor; sin Docker. Vercel + Supabase queda como plan B.
- Cero IA dentro de Folio. Ollama sale. La única IA es el botón "Copiar para Claude" en Escritos, que usa el plan de la organización fuera de Folio. MCP queda para después.
- Login: se restaura el de sesiones propio que ya existía (`internal/middleware/auth.go` `SessionAuth`, `internal/handlers/auth.go`, y `LoginForm.tsx` + `login.astro` desde git). Usuarios creados por script; nada de `admin/admin`.
- Blog: Wix por API; otros por exportación HTML/Markdown.

### Lista mínima, en el orden del camino

| # | Pieza | Estado | Tanda |
|---|---|---|---|
| 1 | Limpieza: fuera app de Mac (Wails, Postgres embebido), Ollama, Inteligencia/chat, Research, Analytics, grafo de entidades, Telegram. Un solo router. | HECHO | 1 |
| 2 | Login con sesiones, usuarios por script | HECHO | 1 |
| 3 | Ingesta RSS sin IA (resumen = descripción del medio) | HECHO | 1 |
| 3b | Diseño Material 3 Expressive en todo el frontend (ver «Diseño») | FALTA | 2 |
| 4 | Servidor de Oracle creado, con Postgres y las migraciones aplicadas | FALTA — Oracle sin capacidad (ver Estado) | 3 |
| 5 | Despliegue en el servidor: binario, Caddy con HTTPS, cron de ingesta, respaldo nocturno | FALTA | 3 |
| 6 | Vigilancias sin IA: marcas, personas y temas por términos; resumen diario | FALTA | 4 |
| 7 | Fondos: APIs de Grants.gov y SAM.gov; crawler de PR con PDF, extracción por patrones y confirmación con un clic | FALTA | 5 |
| 8 | Páginas fijas diarias con lo que cambió | FALTA | 5 |
| 9 | Guardados con carpetas por tema y citas | FALTA | 6 |
| 10 | Escritos: editor, paquete de fuentes, lista SEO/GEO, "Copiar para Claude" | FALTA | 6 |
| 11 | Enviar por email y publicar a Wix | FALTA | 7 |

**Esta obra cubre las piezas 1 a 5 (incluida la 3b).** Las demás son obras siguientes.

### Diseño (pedido de Saul, 14 nov 2026)

- Material 3 Expressive en todo, con animaciones y estilo: formas variadas, resortes, botones que cambian de forma, transición entre páginas, listas segmentadas en vez de cajas con borde.
- Se mantienen los colores de Folio: **blanco, violeta, naranja y gris.** Hoy el código mezcla `indigo` (341 usos), `violet` (73), `amber` (90) y `orange` (4): se unifica a violeta como primario y naranja como acento, sobre fichas de tema, no clases sueltas.
- Letra gruesa (450/600/750), títulos fuertes, contraste medio en modo claro, botón claro/oscuro en todas las páginas, `prefers-reduced-motion` respetado.
- Baldosas y tarjetas que se tocan antes que listas largas; una acción grande por pantalla.
- Referencia de implementación: `~/Downloads/_Projects/celebrate-studio/src/styles/base.css` y `scripts/tema.mjs`.

### PDF de subastas y RFP (pieza 7)

Las convocatorias de municipios y gobierno central viven en PDF, no en la página. Sin IA:

1. El crawler sigue cada enlace a PDF de las páginas vigiladas (por extensión y por `Content-Type`), lo descarga y guarda el archivo original con su huella.
2. Extrae el texto con una librería, sin IA. Si el PDF es escaneado (sin texto), pasa por OCR; si el OCR no está disponible, queda marcado «escaneado» para leerlo a mano.
3. Sobre el texto, patrones: número de subasta o RFP, fecha límite, fecha de reunión pre-subasta, monto, correo y teléfono de contacto.
4. Sale como «posible fondo» con el PDF a la vista y los datos encontrados resaltados. Una persona confirma o descarta.
5. El PDF queda adjunto al fondo y entra en la búsqueda. Un PDF nuevo en la misma página (enmienda, addendum) genera aviso.

Por probar con PDF reales antes de prometer nada: la calidad de la librería de texto y cuántos documentos llegan escaneados. El OCR necesita un binario (Tesseract), lo que obliga a desplegar esa parte como contenedor.

### Lo que NO se construye

- Servidor MCP ni llamadas a ninguna API de IA.
- App de escritorio.
- Bot de Telegram, Research, Analytics, grafo de entidades, chat.
- Archivo de evidencia en S3.
- Fuentes de EE. UU. con filtro de relevancia por IA.

### Terminado, por pieza

- **1–3:** `go build ./... && go vet ./...` limpio sin el paquete `internal/ai`; `npx tsc --noEmit` sin errores; se corre el binario local, se entra con login y el Inbox muestra noticias traídas en esa corrida.
- **3b:** se abren Inbox, Guardados, Vigilancia y Fondos en claro y en oscuro; solo se ven blanco, violeta, naranja y gris; las tarjetas entran con resorte y el cambio de página tiene transición.
- **4:** se entra por SSH al servidor y las migraciones están registradas en `_migrations`.
- **5:** se abre la dirección del servidor por HTTPS, pide login, se entra, y el Inbox muestra noticias traídas por el cron, no por una corrida manual.

### Presupuesto (equivalencias en precio de API, no cargos)

| Partida | Modelo | Estimado |
|---|---|---|
| Orquestador, ~2.5 h | Opus | $11 |
| Tanda 1 · backend: quitar IA de ingesta y handlers, router único, login | Opus | $5 |
| Tanda 1 · frontend: login, quitar páginas, arreglar 3 errores de tipos | Sonnet | $3 |
| Tanda 2 · diseño Expressive: fichas de tema, movimiento, las pantallas que quedan | Sonnet | $5 |
| Tanda 3 · servidor Oracle: binario, Postgres, Caddy, respaldo | Sonnet | $3 |
| Verificadores de cierre (3) | Haiku | $1.50 |
| Reserva para segunda vuelta | — | $11 |
| **Total** | | **~$40, 3 tandas** |

### Archivos compartidos (solo los toca el orquestador)

`cmd/api/main.go` · `frontend/src/lib/api.ts` · `frontend/src/layouts/Layout.astro` · `frontend/src/styles/global.css` · `frontend/tailwind.config.mjs` · `migrations/` · `go.mod` / `go.sum` · `internal/config/config.go` · `vercel.json`

### Por confirmar al abrir la tanda 3

- Que Oracle tenga capacidad (ver Estado). Si sigue sin haber, decidir entre reintentar, pasar la cuenta a pago por uso (los recursos Always Free siguen gratis) o el plan B.
- Dirección pública: dominio propio o DuckDNS, para que Caddy saque el certificado.
- Dónde va el respaldo nocturno de la base.

## Estado

- 14 nov 2026 — Todo el trabajo suelto de marzo quedó en la rama `estado-marzo-2026` (commit `de08f5b`), subida a GitHub. `main` sin tocar.
- 14 nov 2026 — Revisión del código: compila, cero pruebas. Errores conocidos del crawler y de la extracción de oportunidades (se arreglan en la tanda 4, cuando se rehace Fondos): páginas marcadas como revisadas aunque falle la extracción; `parseAmountCents` multiplica por un millón con cualquier "m"; `crawler.go:129` guarda un carácter basura en vez del código HTTP; sin `robots.txt`; sin PDF.
- Datos locales de marzo (1,530 artículos, 21 oportunidades, 361 páginas) siguen en `~/Library/Application Support/Folio/pgdata`. No se migran salvo que Saul lo pida.
- 14 nov 2026 — Oracle: cuenta en prueba gratis, sin instancias ni redes creadas. Se intentó crear `folio` (Ubuntu 24.04, llave `~/.ssh/id_ed25519.pub`) y Oracle respondió **«Out of capacity»** para `VM.Standard.A1.Flex` en AD-1, AD-2 y AD-3, y también para `VM.Standard.E2.1.Micro` en AD-3. No quedó nada creado. Se reintenta al abrir la tanda 3.
- 14 nov 2026 — Arranca la tanda 1 en la rama `obra-1`. Esta tanda el agente de backend es dueño de todo el Go (incluidos `cmd/`, `internal/config`, `migrations/`) y el de frontend de todo `frontend/`; no se cruzan.
- 14 nov 2026 — **Tanda 1 cerrada** en `obra-1` (commit `d3ee17d`). Un solo binario `cmd/folio` (HTTP + cron + frontend y migraciones embebidos), sin IA, con login. Go pasó de ~21,800 a ~11,200 líneas. Primeras pruebas: `internal/crawler`, `internal/scraper`, `internal/watch`.
  - Cómo correrlo: `go build -o folio ./cmd/folio`; variables `DB_*` y `SERVER_PORT` (ver `.env.example`); `FOLIO_PASSWORD='…' ./folio user add correo@org admin`; `./folio`.
  - Verificado por el orquestador con curl contra una base nueva: sin cookie 401; login 200; las 17 rutas GET que usa el frontend 200; las 8 páginas 200; rutas viejas 404; todas las rutas de `frontend/src/lib/api.ts` existen en `cmd/folio/router.go`; en 75 segundos la ingesta inicial trajo 52 artículos, con resumen. **No se abrió en navegador** (regla de Saul): nada se ha visto renderizado.
  - Migración 026: invalida la contraseña de `admin@folio.local`, cierra sus sesiones y solo borra la fila si no tiene datos propios.
  - Pendiente menor: una ruta de página inexistente (p. ej. `/brief`) responde 200 con la página raíz en vez de 404; `/api/logout` con sesión vencida da 401 y no limpia la cookie; `scripts/build-macos.sh` y `docker-compose.yml` siguen describiendo la versión vieja; quedan textos en inglés en el frontend (van con el rediseño); fuentes de EE. UU. como CNN devuelven 403 al scraper y traen páginas de ofertas: revisar la lista de fuentes en la tanda de vigilancias.
  - No verificado: base vieja con datos (la 026 solo se razonó), S3, compilación y ejecución en Linux, crawler/recrawl/refresco de oportunidades en vivo.
- 14 nov 2026 — Oracle: más de diez intentos adicionales, todos «Out of capacity» (A1 en las tres zonas, E2 Micro en AD-3). Quedó creada una red vacía `vcn-20261114-1318`, sin subredes. Alternativa verificada en la documentación de Google Cloud: una `e2-micro` gratis al mes en us-west1, us-central1 o us-east1, 30 GB de disco estándar, 1 GB de salida de red al mes, sin fecha de vencimiento; requiere cuenta de facturación.
- 14 nov 2026 — **Cambio de rumbo en el hospedaje (decisión de Saul): Folio corre en la MacBook de la usuaria, para ella sola.** Nada de servidor, ni Oracle, ni Google, ni Vercel. Esto reemplaza las piezas 4 y 5 y la tanda 3.
  - Por qué la app vieja «no corría» en otra Mac (revisado en `build/Folio.app`): firma ad-hoc sin Developer ID (`spctl` la rechaza: Gatekeeper la bloquea); 13 referencias a `/opt/homebrew` en las librerías del Postgres empaquetado (falla en una Mac sin Homebrew); binario solo arm64 (no abre en Intel); y además exigía Ollama instalado.
  - Tanda 3 nueva, «Folio para Mac»: (1) Postgres embebido con binarios relocalizables —evaluar `fergusstrange/embedded-postgres` antes de escribir nada— y quitar `pgvector` de las migraciones, que ya no se usa; (2) escuchar solo en `127.0.0.1` y mantener el login; (3) `.app` de doble clic que arranca el servidor y abre Folio, más arranque automático al iniciar sesión para que el crawl diario ocurra; (4) firma: Developer ID con notarización, o instalación asistida con «Abrir de todos modos»; (5) respaldo diario de la base a una carpeta; (6) **probar en una cuenta de usuario limpia o en la Mac de ella antes de entregar.**
  - Falta saber: si su MacBook es Apple Silicon o Intel y qué versión de macOS tiene; si hay cuenta de Apple Developer.
- 14 nov 2026 — Tanda 2 (rediseño Material 3 Expressive) en curso con un agente Sonnet sobre `frontend/`.
- 14 nov 2026 — **Obra completa hasta Fondos, en `obra-1` (commit `70e44cc`).** Guardadas: tanda 2 (rediseño M3 Expressive), Folio para Mac (Postgres embebido, solo 127.0.0.1:47615, `/api/setup`, respaldo diario), primer uso y paginación, pantalla de Fondos, y Fondos conectado al crawler (`internal/pdftext`, `internal/extract`, `internal/robots`, `internal/grants`, migración 027). `make mac` produce `build/Folio.app` (94 MB) y `build/Folio.zip` (33 MB). Pruebas en 10 paquetes.
  - Corrida real en entorno limpio (12 min): 106 páginas y 284 PDF (219 con texto, 65 escaneados, 8 rechazados); **163 posibles fondos**: 109 estatales, 53 federales, 1 local, **0 municipales**; 112 con fecha límite, 127 con referencia, **1 con monto (falso)**.
  - Problemas abiertos de calidad: (1) municipios: Caguas solo tenía avisos vencidos y Guaynabo no dio filas; otros municipios sin probar; (2) Grants.gov: los 53 son ruido, ninguno nombra a Puerto Rico en título o agencia; (3) montos casi nunca salen; (4) fechas: un hito del proyecto se tomó como fecha límite en los RFP de CEDD (6 falsos positivos); (5) referencias cortadas duplican una convocatoria; (6) 20 de 28 fondos de PDF son escaneados, solo con el título del enlace; (7) **disco: 527 MB de PDF en una corrida**, falta tope o limpieza.
  - Abierta por primera vez en la Mac de Saul el 14 nov (datos en `~/Library/Application Support/Folio2`). **Nadie había visto el rediseño en pantalla antes de esto.** Falta probar en la MacBook de la usuaria y el paso de Gatekeeper.
  - Sin hacer de `VISION.md`: vigilancias por términos con resumen diario (la base por coincidencia existe), páginas fijas con «qué cambió», carpetas y citas en Guardados, Escritos, email y Wix.
- 16 nov 2026 — Segunda vuelta de Fondos y pedidos de la usuaria, todo en `obra-1` (último `b4f7eae`):
  - Extractor: fecha de entrega vs hitos (CEDD), referencias completas, montos sin penalidades y con techo por premio, enmiendas marcadas, fechas sin año.
  - Grants.gov: solo elegibles para nonprofits, detalle de cada uno, descarta tribus/universidades/gobiernos, tope 25. Ninguno nombra a PR: son programas nacionales abiertos a nonprofits. Palabras clave genéricas hasta saber la misión de la organización.
  - Municipios: solo Caguas, Bayamón y Guaynabo se pueden leer; 12 de 14 grandes publican en imágenes/JS o no responden. El RUS no trae subastas municipales. Ampliar exigiría navegador interno u OCR.
  - Disco: filtro de PDF por tema y antigüedad, topes por dominio y tamaño, HEAD condicional, retención diaria con tope de 1 GB.
  - Noticias: «Borrar» de verdad (oculta al instante, purga a la hora y bloquea la URL), borrado múltiple, Inbox de 7 días y lo no tocado pasa a Archivo para siempre.
  - Fondos: «Guardar» (antes Confirmar), Guardados con filtro Grants/Subastas/RFP, el PDF se conserva con el fondo, «Guardar PDF» suelto, «Descargar PDF», lista de PDF guardados.
  - App reemplazada en la Mac de Saul el 16 nov: respaldo previo `Folio2/respaldos/antes-de-actualizar-2026-11-16.sql.gz` (70 MB), migraciones 028-030 aplicadas, páginas 200 y ruta inexistente 404.
  - Pendiente: misión y marcas de la organización (para Grants.gov y vigilancias); probar en la MacBook de la usuaria y Gatekeeper; nada de esto se ha visto en pantalla salvo lo que Saul mire.

## Para el futuro

### Palabras de tendencias (pedido de Saul, 16 nov 2026; no construir hasta que lo pida)
- Fuente verificada el 16 nov 2026: `https://trends.google.com/trending/rss?geo=PR` (y `geo=US`) responde 200, gratis y sin llave; 10 tendencias diarias con volumen aproximado (`ht:approx_traffic`, p. ej. 500+ a 2000+) y noticias enlazadas. Ese día: «electricidad», «Caguas», «terremoto», «Adamari López», «Objetivo Fama».
- Idea: panel «Lo que se busca hoy en PR» con las tendencias del día, resaltando las que tocan los temas de la organización (educación, juventud, comunidad, vivienda, energía, emprendimiento), historial semanal, y botón «Escribir sobre esto» hacia Escritos (SEO/GEO).
- No disponible gratis: las consultas más buscadas por tema (sección «Explorar» de Google Trends). La API oficial de Trends sería solo por invitación (no verificado); las librerías no oficiales se rompen. Las tendencias diarias son de interés general y rara vez del sector sin fines de lucro.
- Va junto a Escritos, no antes.

### Conectar Meta (Facebook e Instagram) (decisión de Saul, 16 nov 2026: para el futuro)
- Borrador incompleto en `docs/CONECTAR_META.md`.
- Botón «Conectar con Meta» (OAuth) en Ajustes; leer publicaciones, comentarios, menciones y etiquetas de las cuentas propias. Requiere que la organización cree una app en Meta for Developers y que los Instagram sean de empresa vinculados a su página.
- Búsqueda de hashtags de Instagram: requiere Instagram Public Content Access, App Review y verificación de negocio (documentación de Meta, 16 nov 2026); tope 30 hashtags por 7 días.
- Por confirmar: si Meta acepta el redirect a `http://localhost:47615` en modo Desarrollo.
- LinkedIn, X y TikTok: sin acceso viable (socio aprobado, pago o aprobación); quedan como enlaces directos.
