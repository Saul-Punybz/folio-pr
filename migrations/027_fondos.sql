-- Folio Migration 027: Fondos (posibles fondos por patrones, PDF y Grants.gov)
--
-- · opportunities: una página-listado produce varias filas (sin UNIQUE en
--   page_id), los de Grants.gov no tienen dominio (domain_id NULL), cada fila
--   tiene su huella de deduplicado (dedupe_key) y los campos nuevos del
--   contrato: reference, level, source_kind, contact_phone, evidence.
-- · documents: copia local de cada PDF (ruta, sha256, tamaño, escaneado).
-- · opportunity_dismissed: huellas de lo descartado, para que no reaparezca.
-- · crawl_domains: level y start_url (página de subastas/avisos).
-- · crawl_queue: kind (page|pdf) y anchor_text del enlace que lo trajo.
-- · crawled_pages: raw_html para partir listados en filas.
-- · settings: llave/valor simple (palabras clave de Grants.gov).

-- ── Documentos (PDF guardados en <datos>/documentos/<sha256>.pdf) ──
CREATE TABLE IF NOT EXISTS documents (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sha256      TEXT NOT NULL UNIQUE,
    path        TEXT NOT NULL,            -- relativa a la carpeta de datos
    size_bytes  BIGINT NOT NULL DEFAULT 0,
    pages       INT NOT NULL DEFAULT 0,
    scanned     BOOLEAN NOT NULL DEFAULT false,
    source_url  TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Oportunidades ──────────────────────────────────────────────────
ALTER TABLE opportunities DROP CONSTRAINT IF EXISTS opportunities_page_id_key;
ALTER TABLE opportunities ALTER COLUMN domain_id DROP NOT NULL;
ALTER TABLE opportunities DROP CONSTRAINT IF EXISTS opportunities_domain_id_fkey;
ALTER TABLE opportunities ADD CONSTRAINT opportunities_domain_id_fkey
    FOREIGN KEY (domain_id) REFERENCES crawl_domains(id) ON DELETE SET NULL;
ALTER TABLE opportunities DROP CONSTRAINT IF EXISTS opportunities_page_id_fkey;
ALTER TABLE opportunities ADD CONSTRAINT opportunities_page_id_fkey
    FOREIGN KEY (page_id) REFERENCES crawled_pages(id) ON DELETE SET NULL;

ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS reference TEXT NOT NULL DEFAULT '';
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS level TEXT NOT NULL DEFAULT 'estatal';
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS source_kind TEXT NOT NULL DEFAULT 'pagina';
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS contact_phone TEXT NOT NULL DEFAULT '';
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS evidence JSONB NOT NULL DEFAULT '{}';
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS document_id UUID REFERENCES documents(id) ON DELETE SET NULL;
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS document_remote_url TEXT NOT NULL DEFAULT '';
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS document_scanned BOOLEAN NOT NULL DEFAULT false;
-- Campos corregidos a mano: el crawl no los vuelve a pisar.
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS edited_fields TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE opportunities ADD COLUMN IF NOT EXISTS dedupe_key TEXT;

UPDATE opportunities SET dedupe_key = 'legacy:' || id::text WHERE dedupe_key IS NULL;
ALTER TABLE opportunities ALTER COLUMN dedupe_key SET NOT NULL;
ALTER TABLE opportunities DROP CONSTRAINT IF EXISTS opportunities_dedupe_key_key;
ALTER TABLE opportunities ADD CONSTRAINT opportunities_dedupe_key_key UNIQUE (dedupe_key);

ALTER TABLE opportunities DROP CONSTRAINT IF EXISTS opportunities_level_check;
ALTER TABLE opportunities ADD CONSTRAINT opportunities_level_check
    CHECK (level IN ('federal','estatal','municipal','local'));
ALTER TABLE opportunities DROP CONSTRAINT IF EXISTS opportunities_source_kind_check;
ALTER TABLE opportunities ADD CONSTRAINT opportunities_source_kind_check
    CHECK (source_kind IN ('pagina','pdf','api'));

CREATE INDEX IF NOT EXISTS idx_opportunities_reviewed ON opportunities(reviewed, status);
CREATE INDEX IF NOT EXISTS idx_opportunities_level ON opportunities(level);
CREATE INDEX IF NOT EXISTS idx_opportunities_page ON opportunities(page_id);

-- ── Descartados ────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS opportunity_dismissed (
    dedupe_key   TEXT PRIMARY KEY,
    title        TEXT NOT NULL DEFAULT '',
    source_url   TEXT NOT NULL DEFAULT '',
    dismissed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Dominios ───────────────────────────────────────────────────────
ALTER TABLE crawl_domains ADD COLUMN IF NOT EXISTS level TEXT NOT NULL DEFAULT 'estatal';
ALTER TABLE crawl_domains ADD COLUMN IF NOT EXISTS start_url TEXT NOT NULL DEFAULT '';
ALTER TABLE crawl_domains DROP CONSTRAINT IF EXISTS crawl_domains_level_check;
ALTER TABLE crawl_domains ADD CONSTRAINT crawl_domains_level_check
    CHECK (level IN ('federal','estatal','municipal','local'));

-- Gobierno central (estatal) es el valor por defecto. Municipal y local:
UPDATE crawl_domains SET level = 'municipal' WHERE domain IN ('www.crim.org') OR category = 'municipal';
UPDATE crawl_domains SET level = 'local'
 WHERE domain IN ('lumapr.com', 'puertoricobids.net', 'puertorico.grantwatch.com');
-- (recovery.pr.gov, recuperacion.pr.gov y fondosfederales.pr.gov reparten
-- fondos federales, pero quien convoca es una agencia del gobierno central:
-- quedan como estatal. Federal = Grants.gov y agencias federales.)

-- Dominios vigilados para fondos, con su página de subastas/avisos
-- (comprobadas con curl el 14 nov 2026).
INSERT INTO crawl_domains (domain, label, category, max_depth, recrawl_hours, priority, active, level, start_url)
VALUES
  ('subastas.pr.gov', 'Registro Único de Subastas', 'government', 0, 24, 10, true, 'estatal',
     'https://subastas.pr.gov/Pages/subastas.aspx'),
  ('asg.pr.gov', 'Administración de Servicios Generales', 'government', 2, 24, 10, true, 'estatal',
     'https://asg.pr.gov/subastas'),
  ('cedd.pr.gov', 'Consejo Estatal sobre Deficiencias en el Desarrollo', 'government', 1, 48, 10, true, 'estatal',
     'https://cedd.pr.gov/information-y-recursos/rfps'),
  ('www.vivienda.pr.gov', 'Departamento de la Vivienda', 'government', 1, 48, 9, true, 'estatal',
     'https://www.vivienda.pr.gov/aviso-de-solicitudes-de-servicios-profesionales'),
  ('caguas.gov.pr', 'Municipio Autónomo de Caguas', 'municipal', 1, 48, 9, true, 'municipal',
     'https://caguas.gov.pr/avisos-publicos-y-subastas/'),
  ('guaynabocity.gov.pr', 'Municipio de Guaynabo', 'municipal', 1, 48, 8, true, 'municipal',
     'https://guaynabocity.gov.pr/portal-subastas/')
ON CONFLICT (domain) DO UPDATE SET
  level = EXCLUDED.level, start_url = EXCLUDED.start_url, active = true,
  priority = GREATEST(crawl_domains.priority, EXCLUDED.priority),
  max_depth = EXCLUDED.max_depth, recrawl_hours = LEAST(crawl_domains.recrawl_hours, EXCLUDED.recrawl_hours);

UPDATE crawl_domains SET start_url = 'https://www.acueductos.pr.gov/infraestructura/aaa-subastas', priority = 8
 WHERE domain = 'acueductos.pr.gov' AND start_url = '';
UPDATE crawl_domains SET start_url = 'https://recuperacion.pr.gov/avisos-publicos/', priority = 8
 WHERE domain = 'recuperacion.pr.gov' AND start_url = '';
UPDATE crawl_domains SET start_url = 'https://www.prpa.pr.gov/documentos?tipodocumento=Avisos'
 WHERE domain = 'prpa.pr.gov' AND start_url = '';
UPDATE crawl_domains SET start_url = 'https://agricultura.pr.gov/avisos-publicos'
 WHERE domain = 'agricultura.pr.gov' AND start_url = '';
UPDATE crawl_domains SET start_url = 'https://www.familia.pr.gov/documentos?categoria=Convocatorias'
 WHERE domain = 'familia.pr.gov' AND start_url = '';
UPDATE crawl_domains SET start_url = 'https://www.prits.pr.gov/avisos-de-contratacion'
 WHERE domain = 'prits.pr.gov' AND start_url = '';
UPDATE crawl_domains SET start_url = 'https://lumapr.com/suplidores/lista-de-eventos-de-adquisicion/'
 WHERE domain = 'lumapr.com' AND start_url = '';
UPDATE crawl_domains SET start_url = 'https://www.desarrollo.pr.gov/'
 WHERE domain = 'desarrollo.pr.gov' AND start_url = '';
-- cdbg-dr.pr.gov redirige a recuperacion.pr.gov (duplicado); fondosfederales.pr.gov
-- no respondía el 14 nov 2026; GrantWatch responde 403 a robots.
UPDATE crawl_domains SET active = false
 WHERE domain IN ('cdbg-dr.pr.gov', 'fondosfederales.pr.gov', 'puertorico.grantwatch.com');

-- ── Cola ───────────────────────────────────────────────────────────
ALTER TABLE crawl_queue ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'page';
ALTER TABLE crawl_queue ADD COLUMN IF NOT EXISTS anchor_text TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_crawl_queue_pending_depth
    ON crawl_queue (depth ASC, priority DESC, scheduled_at ASC) WHERE status = 'pending';

-- ── Páginas ────────────────────────────────────────────────────────
ALTER TABLE crawled_pages ADD COLUMN IF NOT EXISTS raw_html TEXT NOT NULL DEFAULT '';
ALTER TABLE crawled_pages ADD COLUMN IF NOT EXISTS document_id UUID REFERENCES documents(id) ON DELETE SET NULL;
ALTER TABLE crawled_pages ADD COLUMN IF NOT EXISTS anchor_text TEXT NOT NULL DEFAULT '';
ALTER TABLE crawled_pages ADD COLUMN IF NOT EXISTS source_page_id UUID;
-- Las páginas ya guardadas no tienen raw_html: se revisan de nuevo en el
-- próximo recrawl (el cambio de contenido vuelve a marcar opportunity_checked=false).
CREATE INDEX IF NOT EXISTS idx_crawled_pages_opp_pending
    ON crawled_pages(created_at) WHERE opportunity_checked = false;

-- ── Ajustes ────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- Una palabra clave por línea. Se puede editar con SQL; si falta, se usan las de por defecto.
INSERT INTO settings (key, value) VALUES
  ('grants_keywords', E'Puerto Rico\nnonprofit Puerto Rico\ncommunity development Puerto Rico')
ON CONFLICT (key) DO NOTHING;
