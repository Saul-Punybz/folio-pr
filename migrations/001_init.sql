-- Folio: Political Intelligence System — Initial Schema
-- Run against PostgreSQL 16+ with pgvector extension
--
-- This migration creates the core tables:
--   users, sessions, sources, articles, fingerprints, notes
-- and seeds the default admin user.

-- ── Extensions ───────────────────────────────────────────────
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "vector";

-- ── Users & Sessions ─────────────────────────────────────────

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email         TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('admin', 'member')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE sessions (
    id         TEXT PRIMARY KEY,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

-- ── Sources ──────────────────────────────────────────────────

CREATE TABLE sources (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name           TEXT NOT NULL,
    base_url       TEXT NOT NULL,
    region         TEXT NOT NULL CHECK (region IN ('PR', 'Grants')),
    feed_type      TEXT NOT NULL DEFAULT 'rss' CHECK (feed_type IN ('rss', 'sitemap', 'scrape', 'manual')),
    feed_url       TEXT,
    list_urls      JSONB DEFAULT '[]',
    link_selector  TEXT DEFAULT '',
    title_selector TEXT DEFAULT '',
    body_selector  TEXT DEFAULT '',
    date_selector  TEXT DEFAULT '',
    active         BOOLEAN NOT NULL DEFAULT true,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Articles ─────────────────────────────────────────────────

CREATE TABLE articles (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title               TEXT NOT NULL,
    source              TEXT NOT NULL,
    source_id           UUID REFERENCES sources(id),
    url                 TEXT NOT NULL,
    canonical_url       TEXT,
    region              TEXT NOT NULL CHECK (region IN ('PR', 'Grants')),
    published_at        TIMESTAMPTZ,
    clean_text          TEXT,
    summary             TEXT,
    status              TEXT NOT NULL DEFAULT 'inbox' CHECK (status IN ('inbox', 'saved', 'trashed')),
    pinned              BOOLEAN NOT NULL DEFAULT false,
    evidence_policy     TEXT NOT NULL DEFAULT 'ret_3m' CHECK (evidence_policy IN ('ret_3m', 'ret_6m', 'ret_12m', 'keep')),
    evidence_expires_at TIMESTAMPTZ,
    tags                JSONB DEFAULT '[]',
    embedding           vector(768),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_articles_status ON articles(status);
CREATE INDEX idx_articles_status_created ON articles(status, created_at DESC);
CREATE INDEX idx_articles_region ON articles(region);
CREATE INDEX idx_articles_published_at ON articles(published_at DESC);
CREATE INDEX idx_articles_pinned ON articles(pinned) WHERE pinned = true;
CREATE INDEX idx_articles_evidence_expires ON articles(evidence_expires_at) WHERE evidence_expires_at IS NOT NULL;

-- Full text search index (Spanish — primary language of PR sources)
CREATE INDEX idx_articles_fts ON articles USING GIN (to_tsvector('spanish', coalesce(title, '') || ' ' || coalesce(clean_text, '')));

-- Vector similarity index — uncomment after you have sufficient data (1000+ rows)
-- CREATE INDEX idx_articles_embedding ON articles USING hnsw (embedding vector_cosine_ops);

-- ── Fingerprints (deduplication) ─────────────────────────────

CREATE TABLE fingerprints (
    id                 UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    canonical_url_hash TEXT UNIQUE NOT NULL,
    content_hash       TEXT,
    blocked            BOOLEAN NOT NULL DEFAULT false,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_fingerprints_url_hash ON fingerprints(canonical_url_hash);
CREATE INDEX idx_fingerprints_blocked ON fingerprints(blocked) WHERE blocked = true;

-- ── Notes ────────────────────────────────────────────────────

CREATE TABLE notes (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    article_id UUID NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content    TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_notes_article_id ON notes(article_id);

-- ── Default Admin User ───────────────────────────────────────
-- Password: "admin" — CHANGE THIS IN PRODUCTION
-- bcrypt hash of "admin"
-- Password: "admin" — CHANGE THIS IN PRODUCTION
INSERT INTO users (email, password_hash, role) VALUES
('admin@folio.local', '$2a$10$YhBTIcygV3LJJN7d37MFKOihwjpki7VODvzObXBSaY/xTJr4jlYZS', 'admin');
