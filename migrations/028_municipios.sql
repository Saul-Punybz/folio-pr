-- Folio Migration 028: municipios con página útil y disco de los PDF
--
-- · Municipios: solo entran los dominios cuya página de subastas/avisos se
--   comprobó con curl el 16 nov 2026 y trae avisos que se pueden leer:
--     Bayamón  → lista de avisos en PDF (el texto del enlace es el aviso).
--     Guaynabo → portal de licitadores (Granicus) que publica JSON; la
--                página de portal-subastas solo trae un botón hacia él.
--   El resto (San Juan, Carolina, Ponce, Mayagüez, Toa Baja, Trujillo Alto,
--   Humacao, Aguadilla, Dorado, Cayey, Fajardo, Vega Baja…) no tiene una
--   página legible: ver el informe de la tanda. El Registro Único de Subastas
--   no lista municipios (solo agencias y corporaciones estatales).
-- · Disco: validadores HTTP de cada PDF (ETag/Last-Modified) para no volver a
--   bajarlo, y ajuste del tope total de documentos (1 GB por defecto).

INSERT INTO crawl_domains (domain, label, category, max_depth, recrawl_hours, priority, active, level, start_url)
VALUES
  ('www.municipiodebayamon.com', 'Municipio de Bayamón', 'municipal', 1, 24, 9, true, 'municipal',
     'https://www.municipiodebayamon.com/administracion/avisos-publicos/'),
  ('portalsuplidores.guaynabocity.gov.pr', 'Municipio Autónomo de Guaynabo', 'municipal', 0, 24, 9, true, 'municipal',
     'https://portalsuplidores.guaynabocity.gov.pr/Auction/FillAuctionGrid')
ON CONFLICT (domain) DO UPDATE SET
  level = EXCLUDED.level, start_url = EXCLUDED.start_url, active = true,
  label = EXCLUDED.label, category = EXCLUDED.category,
  priority = GREATEST(crawl_domains.priority, EXCLUDED.priority),
  max_depth = EXCLUDED.max_depth, recrawl_hours = LEAST(crawl_domains.recrawl_hours, EXCLUDED.recrawl_hours);

-- ── PDF ya conocidos ───────────────────────────────────────────────
ALTER TABLE crawled_pages ADD COLUMN IF NOT EXISTS http_etag TEXT NOT NULL DEFAULT '';
ALTER TABLE crawled_pages ADD COLUMN IF NOT EXISTS http_last_modified TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_crawled_pages_document ON crawled_pages(document_id) WHERE document_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_opportunities_document ON opportunities(document_id) WHERE document_id IS NOT NULL;

-- Tope total de la carpeta de documentos, en bytes (1 GB). Editable con SQL.
INSERT INTO settings (key, value) VALUES ('documents_max_bytes', '1073741824')
ON CONFLICT (key) DO NOTHING;
