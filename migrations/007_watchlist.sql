-- Migration 007: Watchlist system (replaces alerts)
-- Adds watchlist_orgs and watchlist_hits tables for NGO monitoring.

-- Drop old alert system
DROP TABLE IF EXISTS alert_matches CASCADE;
DROP TABLE IF EXISTS alerts CASCADE;

-- Watchlist organizations being monitored
CREATE TABLE watchlist_orgs (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name             TEXT NOT NULL,
    website          TEXT NOT NULL DEFAULT '',
    keywords         JSONB NOT NULL DEFAULT '[]',
    youtube_channels JSONB NOT NULL DEFAULT '[]',
    active           BOOLEAN NOT NULL DEFAULT true,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_wo_user ON watchlist_orgs(user_id);
CREATE INDEX idx_wo_active ON watchlist_orgs(active) WHERE active = true;

-- Watchlist hits: mentions found by scanning agents
CREATE TABLE watchlist_hits (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id      UUID NOT NULL REFERENCES watchlist_orgs(id) ON DELETE CASCADE,
    source_type TEXT NOT NULL CHECK (source_type IN ('google_news', 'web', 'local', 'youtube', 'reddit')),
    title       TEXT NOT NULL DEFAULT '',
    url         TEXT NOT NULL,
    url_hash    TEXT NOT NULL,
    snippet     TEXT NOT NULL DEFAULT '',
    sentiment   TEXT NOT NULL DEFAULT 'unknown' CHECK (sentiment IN ('positive', 'neutral', 'negative', 'unknown')),
    ai_draft    TEXT,
    seen        BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_wh_org ON watchlist_hits(org_id);
CREATE UNIQUE INDEX idx_wh_url_hash ON watchlist_hits(url_hash);
CREATE INDEX idx_wh_unseen ON watchlist_hits(seen) WHERE seen = false;
CREATE INDEX idx_wh_created ON watchlist_hits(created_at DESC);
