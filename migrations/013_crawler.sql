-- Folio Migration 013: FolioBot Crawler + Knowledge Graph
-- Adds: crawl_domains, crawl_queue, crawled_pages, crawl_links, crawl_runs,
--        entity_relationships, page_entities tables + seed data

-- ── Crawl Domains ────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS crawl_domains (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain          TEXT NOT NULL UNIQUE,
    label           TEXT NOT NULL DEFAULT '',
    category        TEXT NOT NULL DEFAULT 'other'
                    CHECK (category IN ('government','legislature','executive','municipal','news','federal','other')),
    max_depth       INT NOT NULL DEFAULT 3,
    recrawl_hours   INT NOT NULL DEFAULT 168,
    priority        INT NOT NULL DEFAULT 5 CHECK (priority BETWEEN 1 AND 10),
    active          BOOLEAN NOT NULL DEFAULT true,
    page_count      INT NOT NULL DEFAULT 0,
    last_crawled_at TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Crawl Queue ──────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS crawl_queue (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    url           TEXT NOT NULL,
    url_hash      TEXT NOT NULL UNIQUE,
    domain_id     UUID NOT NULL REFERENCES crawl_domains(id) ON DELETE CASCADE,
    depth         INT NOT NULL DEFAULT 0,
    priority      INT NOT NULL DEFAULT 5 CHECK (priority BETWEEN 1 AND 10),
    status        TEXT NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending','in_progress','done','failed','skipped')),
    discovered_by UUID,
    error_msg     TEXT NOT NULL DEFAULT '',
    attempts      INT NOT NULL DEFAULT 0,
    scheduled_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    claimed_at    TIMESTAMPTZ,
    finished_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crawl_queue_pending
    ON crawl_queue (priority ASC, scheduled_at ASC) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_crawl_queue_domain ON crawl_queue(domain_id);

-- ── Crawled Pages ────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS crawled_pages (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    url             TEXT NOT NULL,
    url_hash        TEXT NOT NULL UNIQUE,
    domain_id       UUID NOT NULL REFERENCES crawl_domains(id) ON DELETE CASCADE,
    title           TEXT NOT NULL DEFAULT '',
    clean_text      TEXT NOT NULL DEFAULT '',
    content_hash    TEXT NOT NULL DEFAULT '',
    summary         TEXT NOT NULL DEFAULT '',
    tags            JSONB NOT NULL DEFAULT '[]',
    entities        JSONB NOT NULL DEFAULT '{}',
    sentiment       TEXT NOT NULL DEFAULT 'unknown',
    embedding       vector(768),
    links_out       INT NOT NULL DEFAULT 0,
    links_in        INT NOT NULL DEFAULT 0,
    depth           INT NOT NULL DEFAULT 0,
    status_code     INT NOT NULL DEFAULT 0,
    content_type    TEXT NOT NULL DEFAULT '',
    content_length  INT NOT NULL DEFAULT 0,
    changed         BOOLEAN NOT NULL DEFAULT false,
    change_summary  TEXT NOT NULL DEFAULT '',
    enriched        BOOLEAN NOT NULL DEFAULT false,
    crawl_count     INT NOT NULL DEFAULT 1,
    first_seen_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_crawled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    next_crawl_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crawled_pages_domain ON crawled_pages(domain_id);
CREATE INDEX IF NOT EXISTS idx_crawled_pages_next_crawl ON crawled_pages(next_crawl_at) WHERE next_crawl_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_crawled_pages_unenriched ON crawled_pages(created_at) WHERE enriched = false;
CREATE INDEX IF NOT EXISTS idx_crawled_pages_changed ON crawled_pages(last_crawled_at DESC) WHERE changed = true;
CREATE INDEX IF NOT EXISTS idx_crawled_pages_fts ON crawled_pages USING gin(to_tsvector('simple', title || ' ' || clean_text));

-- ── Crawl Links ──────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS crawl_links (
    source_page_id UUID NOT NULL REFERENCES crawled_pages(id) ON DELETE CASCADE,
    target_url     TEXT NOT NULL,
    target_hash    TEXT NOT NULL,
    target_page_id UUID REFERENCES crawled_pages(id) ON DELETE SET NULL,
    anchor_text    TEXT NOT NULL DEFAULT '',
    is_external    BOOLEAN NOT NULL DEFAULT false,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (source_page_id, target_hash)
);

CREATE INDEX IF NOT EXISTS idx_crawl_links_target ON crawl_links(target_page_id) WHERE target_page_id IS NOT NULL;

-- ── Crawl Runs ───────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS crawl_runs (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status           TEXT NOT NULL DEFAULT 'running'
                     CHECK (status IN ('running','completed','stopped','failed')),
    pages_crawled    INT NOT NULL DEFAULT 0,
    pages_new        INT NOT NULL DEFAULT 0,
    pages_changed    INT NOT NULL DEFAULT 0,
    pages_enriched   INT NOT NULL DEFAULT 0,
    pages_failed     INT NOT NULL DEFAULT 0,
    domains_visited  INT NOT NULL DEFAULT 0,
    links_discovered INT NOT NULL DEFAULT 0,
    error_msg        TEXT NOT NULL DEFAULT '',
    started_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at      TIMESTAMPTZ
);

-- ── Entity Relationships ─────────────────────────────────────────
CREATE TABLE IF NOT EXISTS entity_relationships (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_entity_id UUID NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    target_entity_id UUID NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    relation_type    TEXT NOT NULL CHECK (relation_type IN (
                         'associated','works_for','heads','member_of',
                         'located_in','funds','opposes','supports','regulates')),
    strength         INT NOT NULL DEFAULT 1,
    evidence_count   INT NOT NULL DEFAULT 1,
    last_seen_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_entity_rels_unique
    ON entity_relationships(source_entity_id, target_entity_id, relation_type);
CREATE INDEX IF NOT EXISTS idx_entity_rels_source ON entity_relationships(source_entity_id);
CREATE INDEX IF NOT EXISTS idx_entity_rels_target ON entity_relationships(target_entity_id);

-- ── Page Entities (junction) ─────────────────────────────────────
CREATE TABLE IF NOT EXISTS page_entities (
    page_id   UUID NOT NULL REFERENCES crawled_pages(id) ON DELETE CASCADE,
    entity_id UUID NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    PRIMARY KEY (page_id, entity_id)
);

-- ── Extend research_findings source_type ─────────────────────────
ALTER TABLE research_findings DROP CONSTRAINT IF EXISTS research_findings_source_type_check;
ALTER TABLE research_findings ADD CONSTRAINT research_findings_source_type_check
    CHECK (source_type IN ('google_news','bing_news','web','local','youtube','reddit','deep_crawl','crawl_index'));

-- ── Seed: PR Government Domains ──────────────────────────────────
INSERT INTO crawl_domains (domain, label, category, max_depth, recrawl_hours, priority) VALUES
    ('www.gobierno.pr',        'Gobierno de PR',           'government',  3, 168, 8),
    ('fortaleza.pr.gov',       'La Fortaleza',             'executive',   3, 72,  9),
    ('www.senado.pr.gov',      'Senado de PR',             'legislature', 3, 72,  9),
    ('www.camara.pr.gov',      'Camara de Representantes', 'legislature', 3, 72,  9),
    ('www.oslpr.org',          'Oficina de Servicios Legislativos', 'legislature', 2, 168, 7),
    ('www.ocpr.gov.pr',        'Oficina del Contralor',    'government',  2, 168, 8),
    ('www.aafaf.pr.gov',       'AAFAF',                    'government',  2, 168, 7),
    ('www.hacienda.pr.gov',    'Depto. de Hacienda',       'government',  2, 168, 7),
    ('www.salud.pr.gov',       'Depto. de Salud',          'government',  2, 168, 6),
    ('www.educacion.pr.gov',   'Depto. de Educacion',      'government',  2, 168, 6),
    ('www.justicia.pr.gov',    'Depto. de Justicia',       'government',  2, 168, 7),
    ('www.trabajo.pr.gov',     'Depto. del Trabajo',       'government',  2, 168, 6),
    ('www.drna.pr.gov',        'Depto. Recursos Naturales','government',  2, 168, 5),
    ('www.crim.org',           'CRIM',                     'municipal',   2, 168, 5),
    ('www.jp.pr.gov',          'Junta de Planificacion',   'government',  2, 168, 6)
ON CONFLICT (domain) DO NOTHING;

-- Seed initial queue entries for each domain's homepage
INSERT INTO crawl_queue (url, url_hash, domain_id, depth, priority)
SELECT
    'https://' || d.domain || '/',
    encode(sha256(('https://' || d.domain || '/')::bytea), 'hex'),
    d.id,
    0,
    d.priority
FROM crawl_domains d
ON CONFLICT (url_hash) DO NOTHING;
