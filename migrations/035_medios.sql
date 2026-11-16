-- 035: Más medios. Revisión de fuentes del 16 nov 2026 (cada feed se probó
-- con curl y con ParseFeed): arregla feeds rotos, reactiva los que ya
-- responden, apaga los que llevan meses sin publicar y suma fuentes nuevas
-- verificadas (con ítems de los últimos 30 días). Idempotente.

-- ── 1. Feeds arreglados ───────────────────────────────────────────

-- TeleOnce: /feed/ viene vacío porque sus noticias son un tipo de contenido
-- propio ("noticias"); el feed de ese tipo sí trae ítems.
UPDATE sources SET feed_type = 'rss', active = true,
    feed_url = 'https://teleonce.com/feed/?post_type=noticias'
WHERE name = 'TeleOnce';

-- News is My Business: /feed/ da 403, pero la API REST de WordPress responde.
-- ParseFeed entiende ese JSON.
UPDATE sources SET feed_type = 'rss', active = true,
    feed_url = 'https://newsismybusiness.com/wp-json/wp/v2/posts?per_page=30&_fields=id,date_gmt,link,title,excerpt,jetpack_featured_media_url'
WHERE name = 'News is My Business';

-- Federal Register: el Atom de búsqueda da 500; el RSS de la API v1 funciona.
UPDATE sources SET feed_type = 'rss', active = true,
    feed_url = 'https://www.federalregister.gov/api/v1/documents.rss?conditions%5Bterm%5D=puerto+rico'
WHERE name = 'Federal Register';

-- ── 2. Reactivadas (ahora responden) ──────────────────────────────

-- WAPA.TV: su portada anuncia un RSS de búsqueda (50 ítems).
UPDATE sources SET feed_type = 'rss', active = true,
    feed_url = 'https://www.wapa.tv/search/?f=rss&t=article&l=50&s=start_time&sd=desc'
WHERE name = 'WAPA.TV';

-- Jay Fonseca: sin RSS, pero su sitemap de Google News trae título y fecha.
UPDATE sources SET feed_type = 'rss', active = true,
    feed_url = 'https://jayfonseca.com/sitemaps/news.xml'
WHERE name = 'Jay Fonseca';

-- ── 3. Apagadas (feed vivo pero sin publicar hace meses) ──────────

UPDATE sources SET active = false
WHERE name IN ('Hacia Donde Vamos',        -- último episodio 2021
               'Inprende',                 -- último post ago 2024
               'CNN',                      -- el feed topstories quedó en 2023
               'Wall Street Journal');     -- feed RSSWorldNews sin cambios desde ene 2025

-- ── 4. Fuentes nuevas verificadas ─────────────────────────────────

INSERT INTO sources (name, base_url, region, feed_type, feed_url, active)
SELECT v.name, v.base_url, v.region, 'rss', v.feed_url, true
FROM (VALUES
    -- Puerto Rico: comunidad, universidades, regional
    ('Fundación Comunitaria de Puerto Rico', 'https://www.fcpr.org',             'PR',      'https://www.fcpr.org/feed/'),
    ('Universidad de Puerto Rico',           'https://www.upr.edu',              'PR',      'https://www.upr.edu/feed/'),
    ('UPR Mayagüez',                         'https://www.uprm.edu',             'PR',      'https://www.uprm.edu/portada/feed/'),
    ('Sin Comillas',                         'https://sincomillas.com',          'PR',      'https://sincomillas.com/feed/'),
    ('80grados',                             'https://www.80grados.net',         'PR',      'https://www.80grados.net/feed/'),
    ('Puerto Rico Report',                   'https://puertoricoreport.com',     'PR',      'https://puertoricoreport.com/feed/'),
    -- Sector sin fines de lucro y filantropía
    ('The Chronicle of Philanthropy',        'https://www.philanthropy.com',     'Grants',  'https://www.philanthropy.com/feed'),
    ('Nonprofit Quarterly',                  'https://nonprofitquarterly.org',   'Grants',  'https://nonprofitquarterly.org/feed/'),
    ('Nonprofit Pro',                        'https://www.nonprofitpro.com',     'Grants',  'https://www.nonprofitpro.com/feed'),
    ('The NonProfit Times',                  'https://www.thenonprofittimes.com','Grants',  'https://www.thenonprofittimes.com/feed/'),
    ('Funds for NGOs',                       'https://www.fundsforngos.org',     'Grants',  'https://www.fundsforngos.org/feed/'),
    ('Independent Sector',                   'https://independentsector.org',    'Grants',  'https://independentsector.org/feed/'),
    ('Nonprofit Finance Fund',               'https://nff.org',                  'Grants',  'https://nff.org/feed'),
    -- Gobierno federal
    ('SBA',                                  'https://www.sba.gov',              'Federal', 'https://www.sba.gov/rss')
) AS v(name, base_url, region, feed_url)
WHERE NOT EXISTS (
    SELECT 1 FROM sources s WHERE s.name = v.name OR s.feed_url = v.feed_url
);

-- ── 5. Noticias de FUERA de PR que hablan de Puerto Rico ──────────
-- Los feeds de portada general de EE. UU. casi nunca mencionan a PR y el
-- filtro de relevancia los descartaba enteros. Se apagan (no se borran) y se
-- sustituyen por fuentes de búsqueda y de etiqueta «Puerto Rico». En las de
-- búsqueda (Google/Bing News) la ingesta asigna la región por el medio real
-- (PR / US / Global) y salta los medios de PR, que ya entran por sus feeds.

UPDATE sources SET active = false
WHERE region = 'US' AND name IN (
    'ABC News', 'Bloomberg', 'CBS News', 'CNN', 'Forbes', 'FOX News',
    'Harvard Business Review', 'NBC News', 'TechCrunch', 'The New York Times',
    'The Rundown AI', 'Wall Street Journal', 'Washington Post', 'Y Combinator News'
);

INSERT INTO sources (name, base_url, region, feed_type, feed_url, active)
SELECT v.name, v.base_url, v.region, 'rss', v.feed_url, true
FROM (VALUES
    ('Google News: Puerto Rico (EN)', 'https://news.google.com', 'Global',
     'https://news.google.com/rss/search?q=%22Puerto%20Rico%22&hl=en-US&gl=US&ceid=US:en'),
    ('Google News: Puerto Rico (ES, fuera de PR)', 'https://news.google.com', 'Global',
     'https://news.google.com/rss/search?q=%22Puerto%20Rico%22&hl=es-419&gl=US&ceid=US:es-419'),
    ('Bing News: Puerto Rico (EN)', 'https://www.bing.com/news', 'Global',
     'https://www.bing.com/news/search?q=%22Puerto+Rico%22&format=rss&setlang=en-US&cc=US'),
    ('The Guardian: Puerto Rico', 'https://www.theguardian.com', 'Global',
     'https://www.theguardian.com/world/puerto-rico/rss'),
    ('The New York Times: Puerto Rico', 'https://www.nytimes.com', 'US',
     'https://www.nytimes.com/svc/collections/v1/publish/https://www.nytimes.com/topic/destination/puerto-rico/rss.xml')
) AS v(name, base_url, region, feed_url)
WHERE NOT EXISTS (
    SELECT 1 FROM sources s WHERE s.name = v.name OR s.feed_url = v.feed_url
);
