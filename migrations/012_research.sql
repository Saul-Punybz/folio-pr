-- Folio Migration 012: Deep Research (Investigacion Profunda)
-- Adds: research_projects, research_findings tables

CREATE TABLE IF NOT EXISTS research_projects (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    topic       TEXT NOT NULL,
    keywords    JSONB NOT NULL DEFAULT '[]',
    status      TEXT NOT NULL DEFAULT 'queued'
                CHECK (status IN ('queued','searching','scraping','synthesizing','done','failed','cancelled')),
    phase       INT NOT NULL DEFAULT 0,
    progress    JSONB NOT NULL DEFAULT '{}',
    dossier     TEXT NOT NULL DEFAULT '',
    entities    JSONB NOT NULL DEFAULT '{}',
    timeline    JSONB NOT NULL DEFAULT '[]',
    error_msg   TEXT NOT NULL DEFAULT '',
    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS research_findings (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   UUID NOT NULL REFERENCES research_projects(id) ON DELETE CASCADE,
    url          TEXT NOT NULL,
    url_hash     TEXT NOT NULL,
    title        TEXT NOT NULL DEFAULT '',
    snippet      TEXT NOT NULL DEFAULT '',
    clean_text   TEXT NOT NULL DEFAULT '',
    source_type  TEXT NOT NULL CHECK (source_type IN (
                     'google_news','bing_news','web','local','youtube','reddit','deep_crawl')),
    sentiment    TEXT NOT NULL DEFAULT 'unknown',
    relevance    REAL NOT NULL DEFAULT 0.0,
    image_url    TEXT,
    published_at TIMESTAMPTZ,
    scraped      BOOLEAN NOT NULL DEFAULT false,
    tags         JSONB NOT NULL DEFAULT '[]',
    entities     JSONB NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_research_findings_project ON research_findings(project_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_research_findings_dedup ON research_findings(project_id, url_hash);
CREATE INDEX IF NOT EXISTS idx_research_findings_relevance ON research_findings(project_id, relevance DESC);
CREATE INDEX IF NOT EXISTS idx_research_findings_source ON research_findings(project_id, source_type);
