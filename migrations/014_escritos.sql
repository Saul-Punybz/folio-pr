-- Folio Migration 014: Escritos (SEO Article Generator)
-- Adds: escritos, escrito_sources tables

CREATE TABLE IF NOT EXISTS escritos (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    topic           TEXT NOT NULL,
    slug            TEXT NOT NULL DEFAULT '',
    title           TEXT NOT NULL DEFAULT '',
    meta_description TEXT NOT NULL DEFAULT '',
    keywords        JSONB NOT NULL DEFAULT '[]',
    hashtags        JSONB NOT NULL DEFAULT '[]',
    content         TEXT NOT NULL DEFAULT '',
    article_plan    JSONB NOT NULL DEFAULT '[]',
    seo_score       JSONB NOT NULL DEFAULT '{}',
    status          TEXT NOT NULL DEFAULT 'queued'
                    CHECK (status IN ('queued','planning','generating','scoring','done','failed')),
    phase           INT NOT NULL DEFAULT 0,
    progress        JSONB NOT NULL DEFAULT '{}',
    publish_status  TEXT NOT NULL DEFAULT 'draft'
                    CHECK (publish_status IN ('draft','reviewing','published')),
    word_count      INT NOT NULL DEFAULT 0,
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS escrito_sources (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    escrito_id  UUID NOT NULL REFERENCES escritos(id) ON DELETE CASCADE,
    article_id  UUID NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    relevance   REAL NOT NULL DEFAULT 0.0,
    used_in_section TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_escritos_user ON escritos(user_id);
CREATE INDEX IF NOT EXISTS idx_escritos_status ON escritos(status);
CREATE INDEX IF NOT EXISTS idx_escrito_sources_escrito ON escrito_sources(escrito_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_escrito_sources_dedup ON escrito_sources(escrito_id, article_id);
