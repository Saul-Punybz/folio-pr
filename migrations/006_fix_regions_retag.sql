-- Migration 006: Fix region constraints, clean garbage tags, fix RSS URLs
-- Run: /opt/homebrew/Cellar/postgresql@17/17.9/bin/psql -U folio -d folio -f migrations/006_fix_regions_retag.sql

-- ── 1. Fix region CHECK constraints ────────────────────────────
-- Widen allowed regions from ('PR','Grants') to ('PR','Grants','Federal','Local')

ALTER TABLE sources DROP CONSTRAINT IF EXISTS sources_region_check;
ALTER TABLE sources ADD CONSTRAINT sources_region_check
    CHECK (region IN ('PR', 'Grants', 'Federal', 'Local'));

ALTER TABLE articles DROP CONSTRAINT IF EXISTS articles_region_check;
ALTER TABLE articles ADD CONSTRAINT articles_region_check
    CHECK (region IN ('PR', 'Grants', 'Federal', 'Local'));

-- ── 2. Clean garbage AI tags ───────────────────────────────────
-- Null out tags that are garbage (long strings, Spanish paragraphs, etc.)
-- so re-enrichment picks them up

UPDATE articles
SET tags = NULL, summary = NULL
WHERE tags IS NOT NULL AND (
    -- Tags array contains items longer than 30 chars (garbage paragraphs)
    EXISTS (
        SELECT 1 FROM jsonb_array_elements_text(tags) AS t
        WHERE length(t) > 30
    )
    -- Or summary contains known garbage patterns
    OR lower(summary) LIKE '%no hay información%'
    OR lower(summary) LIKE '%no tengo%'
    OR lower(summary) LIKE '%no puedo%'
    OR lower(summary) LIKE '%i cannot%'
    OR lower(summary) LIKE '%clasificarlo en%'
    OR lower(summary) LIKE '%posibles etiquetas%'
);

-- Also null out empty tags arrays so they get re-enriched
UPDATE articles SET tags = NULL WHERE tags = '[]'::jsonb;

-- ── 3. Fix broken RSS feed URLs ───────────────────────────────

-- Federal Register: fix redirect + set region
UPDATE sources
SET feed_url = 'https://www.federalregister.gov/api/v1/documents.rss',
    region = 'Federal'
WHERE lower(name) LIKE '%federal register%';

-- GovInfo: fix HTML page → actual RSS
UPDATE sources
SET feed_url = 'https://www.govinfo.gov/rss/fr.xml',
    region = 'Federal'
WHERE lower(name) LIKE '%govinfo%';

-- Grants.gov: deactivate — RSS deprecated, site redesigned
UPDATE sources
SET active = false
WHERE lower(name) LIKE '%grants.gov%';

-- ── 4. Convert scrape sources to RSS where WordPress feeds exist ─

-- CPI (Centro de Periodismo Investigativo)
UPDATE sources
SET feed_type = 'rss',
    feed_url = 'https://periodismoinvestigativo.com/feed/'
WHERE lower(name) LIKE '%cpi%' OR lower(name) LIKE '%periodismo investigativo%';

-- News is My Business
UPDATE sources
SET feed_type = 'rss',
    feed_url = 'https://newsismybusiness.com/feed/'
WHERE lower(name) LIKE '%news is my business%';

-- Es Noticia PR
UPDATE sources
SET feed_type = 'rss',
    feed_url = 'https://esnoticiapr.com/feed/'
WHERE lower(name) LIKE '%es noticia%';

-- TeleOnce
UPDATE sources
SET feed_type = 'rss',
    feed_url = 'https://teleonce.com/feed/'
WHERE lower(name) LIKE '%teleonce%';

-- Radio Isla
UPDATE sources
SET feed_type = 'rss',
    feed_url = 'https://radioisla.tv/feed/'
WHERE lower(name) LIKE '%radio isla%';

-- Done
