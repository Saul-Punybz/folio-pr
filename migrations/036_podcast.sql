-- Menciones: fuente nueva «podcast» (episodios de Apple Podcasts que nombran la marca).
ALTER TABLE watchlist_hits DROP CONSTRAINT IF EXISTS watchlist_hits_source_type_check;
ALTER TABLE watchlist_hits ADD CONSTRAINT watchlist_hits_source_type_check
    CHECK (source_type IN ('google_news', 'bing_news', 'web', 'local', 'youtube', 'reddit', 'bluesky', 'sitio', 'mastodon', 'alertas', 'podcast'));
