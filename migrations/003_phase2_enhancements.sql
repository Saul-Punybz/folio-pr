-- Phase 2: Search enhancements, embeddings, similarity search, clustering
--
-- This migration adds indexes for bilingual FTS, vector similarity, and tag filtering.

-- Add English FTS index alongside the existing Spanish one.
-- The 'simple' config is used at query time for bilingual support; this index
-- helps if queries target English-specific stemming.
CREATE INDEX IF NOT EXISTS idx_articles_fts_en ON articles USING GIN (to_tsvector('english', coalesce(title, '') || ' ' || coalesce(clean_text, '')));

-- Add 'simple' FTS index for bilingual queries (no language-specific stemming).
CREATE INDEX IF NOT EXISTS idx_articles_fts_simple ON articles USING GIN (to_tsvector('simple', coalesce(title, '') || ' ' || coalesce(clean_text, '')));

-- Add HNSW index for vector similarity search (cosine distance).
-- Works with any number of rows; build time increases with data volume.
CREATE INDEX IF NOT EXISTS idx_articles_embedding ON articles USING hnsw (embedding vector_cosine_ops);

-- Add GIN index on tags JSONB column for efficient tag filtering.
CREATE INDEX IF NOT EXISTS idx_articles_tags ON articles USING GIN (tags);
