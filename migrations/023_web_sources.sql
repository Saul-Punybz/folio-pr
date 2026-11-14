-- Folio Migration 023: Web Sources for Escritos
-- Allows escrito_sources to reference web-researched pages (no local article).

-- Make article_id nullable (web sources have no local article).
ALTER TABLE escrito_sources ALTER COLUMN article_id DROP NOT NULL;

-- Add columns for web source metadata (used when article_id IS NULL).
ALTER TABLE escrito_sources ADD COLUMN IF NOT EXISTS web_url    TEXT NOT NULL DEFAULT '';
ALTER TABLE escrito_sources ADD COLUMN IF NOT EXISTS web_title  TEXT NOT NULL DEFAULT '';
ALTER TABLE escrito_sources ADD COLUMN IF NOT EXISTS web_domain TEXT NOT NULL DEFAULT '';

-- Replace the old unique index (doesn't handle NULLs) with a partial one.
DROP INDEX IF EXISTS idx_escrito_sources_dedup;
CREATE UNIQUE INDEX IF NOT EXISTS idx_escrito_sources_dedup
    ON escrito_sources(escrito_id, article_id) WHERE article_id IS NOT NULL;
