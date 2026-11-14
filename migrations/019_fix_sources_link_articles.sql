-- Migration 019: Deduplicate sources and link orphaned articles
--
-- 1. Remove duplicate sources (keep the oldest entry per name+region)
-- 2. Set source_id on all articles by matching articles.source to sources.name

-- Step 1: Delete duplicate sources, keeping the earliest created per name+region
DELETE FROM sources
WHERE id NOT IN (
    SELECT DISTINCT ON (name, region) id
    FROM sources
    ORDER BY name, region, created_at ASC
);

-- Step 2: Link all orphaned articles to their source by matching the text name
UPDATE articles a
SET source_id = s.id
FROM sources s
WHERE a.source = s.name
  AND a.source_id IS NULL;
