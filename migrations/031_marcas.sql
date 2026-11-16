-- Migration 031: Marcas.
-- watchlist_orgs pasa a guardar tres tipos de cosa: marcas de la organización
-- (con alias, exclusiones, hashtags y perfiles en redes), temas y personas.
-- Las plantillas de marca se siembran en Go al arrancar (user_id es NOT NULL).

ALTER TABLE watchlist_orgs
    ADD COLUMN IF NOT EXISTS kind             TEXT    NOT NULL DEFAULT 'tema',
    ADD COLUMN IF NOT EXISTS aliases          JSONB   NOT NULL DEFAULT '[]',
    ADD COLUMN IF NOT EXISTS exclude_terms    JSONB   NOT NULL DEFAULT '[]',
    ADD COLUMN IF NOT EXISTS hashtags         JSONB   NOT NULL DEFAULT '[]',
    ADD COLUMN IF NOT EXISTS socials          JSONB   NOT NULL DEFAULT '[]',
    ADD COLUMN IF NOT EXISTS to_confirm       BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS sources_evidence JSONB   NOT NULL DEFAULT '[]',
    -- Feeds RSS de alertas (p. ej. Google Alerts «Entregar a: Feed RSS»).
    ADD COLUMN IF NOT EXISTS alert_feeds      JSONB   NOT NULL DEFAULT '[]',
    -- Alias genéricos («Platform for Social Impact», «PSI»): solo cuentan
    -- junto a Puerto Rico u otro nombre de la organización.
    ADD COLUMN IF NOT EXISTS context_aliases  JSONB   NOT NULL DEFAULT '[]',
    -- Personas clave: [{name, role, url}]; su nombre también es mención.
    ADD COLUMN IF NOT EXISTS people           JSONB   NOT NULL DEFAULT '[]';

ALTER TABLE watchlist_orgs DROP CONSTRAINT IF EXISTS watchlist_orgs_kind_check;
ALTER TABLE watchlist_orgs ADD CONSTRAINT watchlist_orgs_kind_check
    CHECK (kind IN ('marca', 'tema', 'persona'));

CREATE INDEX IF NOT EXISTS idx_wo_user_kind ON watchlist_orgs(user_id, kind);

-- Menciones: fuentes nuevas (bluesky, sitio, mastodon, alertas). bing_news faltaba en
-- la lista original y sus menciones se perdían en silencio.
ALTER TABLE watchlist_hits DROP CONSTRAINT IF EXISTS watchlist_hits_source_type_check;
ALTER TABLE watchlist_hits ADD CONSTRAINT watchlist_hits_source_type_check
    CHECK (source_type IN ('google_news', 'bing_news', 'web', 'local', 'youtube', 'reddit', 'bluesky', 'sitio', 'mastodon', 'alertas'));

ALTER TABLE watchlist_hits
    ADD COLUMN IF NOT EXISTS network      TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS matched_hashtag TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS matched_person  TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS published_at    TIMESTAMPTZ,
    -- Título normalizado: la misma noticia por Google News, Bing y la ingesta
    -- (con URL distinta) cuenta una sola vez por marca.
    ADD COLUMN IF NOT EXISTS title_key       TEXT NOT NULL DEFAULT '';

UPDATE watchlist_hits SET network = CASE source_type
    WHEN 'google_news' THEN 'noticias'
    WHEN 'bing_news'   THEN 'noticias'
    WHEN 'local'       THEN 'noticias'
    ELSE source_type END
WHERE network = '';

CREATE INDEX IF NOT EXISTS idx_wh_org_title ON watchlist_hits(org_id, title_key) WHERE title_key <> '';
CREATE INDEX IF NOT EXISTS idx_wh_org_seen ON watchlist_hits(org_id) WHERE seen = false;
