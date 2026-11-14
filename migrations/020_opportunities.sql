-- Folio Migration 020: Opportunity Tracker
-- Adds opportunities table for extracted grant/RFP data from crawled government pages.

CREATE TABLE IF NOT EXISTS opportunities (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    page_id         UUID UNIQUE REFERENCES crawled_pages(id) ON DELETE CASCADE,
    domain_id       UUID NOT NULL REFERENCES crawl_domains(id) ON DELETE CASCADE,
    title           TEXT NOT NULL DEFAULT '',
    agency          TEXT NOT NULL DEFAULT '',
    summary         TEXT NOT NULL DEFAULT '',
    amount          TEXT NOT NULL DEFAULT '',
    amount_cents    BIGINT NOT NULL DEFAULT 0,
    deadline        DATE,
    contact_name    TEXT NOT NULL DEFAULT '',
    contact_email   TEXT NOT NULL DEFAULT '',
    category        TEXT NOT NULL DEFAULT 'other'
                    CHECK (category IN ('rfp','grant','funding','proposal','contract','fellowship','scholarship','other')),
    status          TEXT NOT NULL DEFAULT 'unknown'
                    CHECK (status IN ('open','closing_soon','closed','unknown')),
    source_url      TEXT NOT NULL DEFAULT '',
    confidence      INT NOT NULL DEFAULT 0 CHECK (confidence BETWEEN 0 AND 100),
    raw_extraction  JSONB NOT NULL DEFAULT '{}',
    reviewed        BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_opportunities_status ON opportunities(status);
CREATE INDEX IF NOT EXISTS idx_opportunities_category ON opportunities(category);
CREATE INDEX IF NOT EXISTS idx_opportunities_deadline ON opportunities(deadline) WHERE deadline IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_opportunities_domain ON opportunities(domain_id);
CREATE INDEX IF NOT EXISTS idx_opportunities_fts ON opportunities
    USING gin(to_tsvector('simple', title || ' ' || agency || ' ' || summary));

-- Add opportunity_checked flag to crawled_pages
ALTER TABLE crawled_pages ADD COLUMN IF NOT EXISTS opportunity_checked BOOLEAN NOT NULL DEFAULT false;
CREATE INDEX IF NOT EXISTS idx_crawled_pages_opp_unchecked
    ON crawled_pages(created_at) WHERE opportunity_checked = false AND enriched = true;
