# Escritos Continuation

Estado al 2026-03-17

Ya implementado:
- Perfil editorial persistente en `escritos`:
  - `output_format`
  - `writing_style`
  - `target_audience`
  - `citation_style`
- Generacion extendida a 5 fases:
  - research
  - planning
  - generation
  - scoring
  - verification
- Verificador post-generacion en `internal/generator/generator.go`:
  - estructura minima por formato
  - bibliografia / referencias
  - markdown exportable
  - deteccion basica de afirmaciones sin soporte explicito
- Export `.md` mejorado:
  - frontmatter
  - lista de fuentes verificadas
  - mejor separacion de metadata / contenido / referencias
- Estado `verifying` agregado al pipeline y a la UI
- Default de Ollama alineado con modelos locales instalados:
  - `llama3.2:3b`
  - `nomic-embed-text`
- `scripts/build-macos.sh` ya no falla todo el build si el DMG no se puede crear
- Correccion de UI en Escritos:
  - ya no muestra `Fase 5/4`
  - polling incluye `improving`
  - mientras mejora/optimiza, el usuario sigue viendo el articulo actual

Bloque nuevo completado hoy:
- Generacion mas larga y profunda:
  - `runPhase2` ahora pide targets mas altos por formato
  - `defaultPlan()` ahora crea mas secciones y targets bastante mayores
  - `normalizeSectionTargets()` fuerza minima profundidad total
  - `generateSection()` ahora exige mas densidad informativa y desarrollo real
- Mejora mas fuerte de contenido:
  - `ImproveContent()` conserva mas contexto (`24000` chars en vez de `12000`)
  - metadata se regenera despues de mejorar
  - score SEO se recalcula despues de mejorar
- Nueva expansion explicita:
  - nuevo endpoint `POST /api/escritos/{id}/expand`
  - `generator.ExpandContent()` hace expansion long-form real
  - UI nueva con boton `Expandir Articulo`
- SEO tab ahora diferencia:
  - `Recalcular Puntaje`
  - `Optimizar SEO`
  - `Expandir Articulo`
  - presets rapidos para claridad / estructura / keyword / expansion

Archivos tocados:
- `migrations/025_escritos_verifying_status.sql`
- `internal/config/config.go`
- `internal/generator/generator.go`
- `internal/handlers/escritos.go`
- `cmd/api/main.go`
- `cmd/app/main.go`
- `frontend/src/lib/api.ts`
- `frontend/src/islands/EscritosDesk.tsx`
- `scripts/build-macos.sh`

Validacion ya corrida:
- `go test ./internal/generator ./internal/handlers ./cmd/api ./cmd/app`
- `npm run build`
- `bash scripts/build-macos.sh`

Estado operativo actual:
- `Folio.app` compila bien
- `ollama serve` estaba requerido porque Phase 2 fallaba por `localhost:11434`
- el app puede caerse al lanzar si queda un postgres embebido huerfano con `postmaster.pid`
- cuando eso pase:
  - revisar `pgrep -fl 'postgres .*Application Support/Folio/pgdata'`
  - matar proceso huerfano
  - borrar `~/Library/Application Support/Folio/pgdata/postmaster.pid`
  - relanzar `Folio.app`

Proximo bloque recomendado:
1. Mostrar `verification report` en el tab SEO
   - checks
   - issues
   - warnings
2. Hacer expansion por seccion, no solo rewrite completo
   - expandir FAQ
   - expandir conclusion
   - expandir contexto / impacto / riesgos
3. Permitir “regenerar solo una seccion”
4. Mejorar calidad de referencias
   - mas fuentes oficiales
   - mejor agrupacion academica / gobierno / medios
5. Hacer que la UI muestre delta despues de optimizar/expandir
   - score antes/despues
   - palabras antes/despues
   - metadata antes/despues
6. Investigar fallo intermitente de Astro al correr `npm run build` directo
   - a veces falta `renderers.mjs`
   - `build-macos.sh` ha seguido completando

Si retomas con poco contexto:
- primero abre `internal/generator/generator.go`
- luego `internal/handlers/escritos.go`
- despues `frontend/src/islands/EscritosDesk.tsx`
- finalmente corre:
  - `go test ./internal/generator ./internal/handlers ./cmd/api ./cmd/app`
  - `npm run build`
  - `bash scripts/build-macos.sh`
