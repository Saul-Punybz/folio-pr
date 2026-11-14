-- Phase 2: Search enhancements (bilingual FTS, tag filtering)
--
-- This migration adds indexes for bilingual FTS and tag filtering.

-- Add English FTS index alongside the existing Spanish one.
-- The 'simple' config is used at query time for bilingual support; this index
-- helps if queries target English-specific stemming.
CREATE INDEX IF NOT EXISTS idx_articles_fts_en ON articles USING GIN (to_tsvector('english', coalesce(title, '') || ' ' || coalesce(clean_text, '')));

-- Add 'simple' FTS index for bilingual queries (no language-specific stemming).
CREATE INDEX IF NOT EXISTS idx_articles_fts_simple ON articles USING GIN (to_tsvector('simple', coalesce(title, '') || ' ' || coalesce(clean_text, '')));

-- Add GIN index on tags JSONB column for efficient tag filtering.
CREATE INDEX IF NOT EXISTS idx_articles_tags ON articles USING GIN (tags);
