-- Folio Migration 010: Entity Graph + Trend Tracking
-- Adds: entities table, article_entities junction, articles.entities JSONB, articles.sentiment

-- Deduplicated entity registry
CREATE TABLE IF NOT EXISTS entities (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL,
    type       TEXT NOT NULL CHECK (type IN ('person', 'organization', 'place')),
    canonical  TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_entities_canonical_type ON entities(canonical, type);

-- Many-to-many: which entities appear in which articles
CREATE TABLE IF NOT EXISTS article_entities (
    article_id UUID NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    entity_id  UUID NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    PRIMARY KEY (article_id, entity_id)
);
CREATE INDEX IF NOT EXISTS idx_ae_entity ON article_entities(entity_id);
CREATE INDEX IF NOT EXISTS idx_ae_article ON article_entities(article_id);

-- Add entities JSONB column on articles for quick access (denormalized)
ALTER TABLE articles ADD COLUMN IF NOT EXISTS entities JSONB DEFAULT '{}';

-- Add article-level sentiment
ALTER TABLE articles ADD COLUMN IF NOT EXISTS sentiment TEXT DEFAULT '' CHECK (sentiment IN ('', 'positive', 'neutral', 'negative'));

-- Fix sources: switch 6 sources from scrape to RSS (confirmed working feeds)
UPDATE sources SET feed_type = 'rss', feed_url = 'https://www.noticel.com/feed/',
    link_selector = '', title_selector = '', body_selector = '', date_selector = ''
WHERE name = 'NotiCel' AND feed_type = 'scrape';

UPDATE sources SET feed_type = 'rss', feed_url = 'https://radioisla.tv/feed/',
    link_selector = '', title_selector = '', body_selector = '', date_selector = ''
WHERE name = 'Radio Isla 1320' AND feed_type = 'scrape';

UPDATE sources SET feed_type = 'rss', feed_url = 'https://newsismybusiness.com/feed/',
    link_selector = '', title_selector = '', body_selector = '', date_selector = ''
WHERE name = 'News is My Business' AND feed_type = 'scrape';

UPDATE sources SET feed_type = 'rss', feed_url = 'https://esnoticiapr.com/feed/',
    link_selector = '', title_selector = '', body_selector = '', date_selector = ''
WHERE name = 'Es Noticia PR' AND feed_type = 'scrape';

-- Fix GovInfo: set correct RSS feed URL
UPDATE sources SET feed_url = 'https://www.govinfo.gov/rss/fr.xml'
WHERE name = 'GovInfo';

-- Re-enable Federal Register with Atom feed (parser already handles Atom)
UPDATE sources SET active = true, feed_url = 'https://www.federalregister.gov/documents/search.atom?conditions%5Bterm%5D=puerto+rico'
WHERE name = 'Federal Register';

-- Configure scrape sources with CSS selectors

-- Primera Hora
UPDATE sources SET
    list_urls = '["https://www.primerahora.com/noticias/", "https://www.primerahora.com/noticias/gobierno-politica/"]',
    link_selector = 'article a[href*="/noticias/"], .story-tease a',
    title_selector = 'h1.headline, h1',
    body_selector = '.article-body p, .story-body p',
    date_selector = 'time[datetime], .date-published',
    active = true
WHERE name = 'Primera Hora';

-- El Vocero
UPDATE sources SET
    list_urls = '["https://www.elvocero.com/gobierno/", "https://www.elvocero.com/economia/"]',
    link_selector = '.tnt-headline a, h3 a[href*="/gobierno/"], h3 a[href*="/economia/"]',
    title_selector = 'h1.headline, h1.tnt-headline',
    body_selector = '.asset-content p, .tnt-body p',
    date_selector = 'time[datetime], .tnt-date',
    active = true
WHERE name = 'El Vocero';

-- Metro Puerto Rico
UPDATE sources SET
    list_urls = '["https://www.metro.pr/noticias/"]',
    link_selector = 'article a[href*="/noticias/"], .entry-title a',
    title_selector = 'h1.entry-title, h1',
    body_selector = '.entry-content p, .post-content p',
    date_selector = 'time[datetime], .entry-date',
    active = true
WHERE name = 'Metro Puerto Rico';

-- WAPA.TV
UPDATE sources SET
    list_urls = '["https://www.wapa.tv/noticias"]',
    link_selector = 'a[href*="/noticias/"]',
    title_selector = 'h1',
    body_selector = '.article-body p, .content-body p, .field-body p',
    date_selector = 'time[datetime], .date',
    active = true
WHERE name = 'WAPA.TV';

-- El Calce
UPDATE sources SET
    list_urls = '["https://www.elcalce.com/"]',
    link_selector = 'article a, .post-title a, h2 a',
    title_selector = 'h1.entry-title, h1.post-title, h1',
    body_selector = '.entry-content p, .post-content p',
    date_selector = 'time[datetime], .entry-date, .post-date',
    active = true
WHERE name = 'El Calce';

-- Senado de Puerto Rico
UPDATE sources SET
    list_urls = '["https://senado.pr.gov/Pages/Communications.aspx"]',
    link_selector = 'a[href*="Communications"], a[href*="comunicado"], .ms-vb a',
    title_selector = 'h1, .page-header h1',
    body_selector = '.ms-rtestate-field p, .ExternalClass p, article p',
    date_selector = '.ms-vb, time',
    active = true
WHERE name = 'Senado de Puerto Rico';

-- Camara de Representantes
UPDATE sources SET
    list_urls = '["https://tucamarapr.org/noticias/"]',
    link_selector = 'article a, .entry-title a, h2 a[href*="/noticias/"]',
    title_selector = 'h1.entry-title, h1',
    body_selector = '.entry-content p, .post-content p',
    date_selector = 'time[datetime], .entry-date',
    active = true
WHERE name = 'Camara de Representantes';

-- La Fortaleza
UPDATE sources SET
    list_urls = '["https://www.fortaleza.pr.gov/comunicados"]',
    link_selector = 'a[href*="comunicado"], article a, .views-row a',
    title_selector = 'h1.page-title, h1',
    body_selector = '.field--name-body p, article p, .node-content p',
    date_selector = 'time[datetime], .date-display-single',
    active = true
WHERE name = 'La Fortaleza';

-- Disable broken sources
UPDATE sources SET active = false WHERE name = 'Jay Fonseca';
UPDATE sources SET active = false WHERE name = 'Noticentro (WAPA)';
UPDATE sources SET active = false WHERE name = 'Caribbean Business';

-- Grants.gov: disable (XML endpoint returns HTML)
UPDATE sources SET active = false WHERE name = 'Grants.gov';
