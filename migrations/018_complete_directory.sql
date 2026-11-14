-- Migration 018: Complete media directory — INSERT missing sources, fix broken feeds
-- Only uses verified working RSS feeds (tested 2026-03-10)

-- ══════════════════════════════════════════════════════════════
-- PART 1: Fix existing sources with broken RSS (revert to scrape, deactivate)
-- Migration 017 guessed RSS URLs that don't exist.
-- ══════════════════════════════════════════════════════════════

-- Government sites have no RSS feeds
UPDATE sources SET feed_type = 'scrape', feed_url = NULL, active = false
WHERE name = 'Camara de Representantes';

UPDATE sources SET feed_type = 'scrape', feed_url = NULL, active = false
WHERE name = 'Senado de Puerto Rico';

UPDATE sources SET feed_type = 'scrape', feed_url = NULL, active = false
WHERE name = 'La Fortaleza';

-- These feeds return 404 or timeout
UPDATE sources SET feed_type = 'scrape', feed_url = NULL, active = false
WHERE name = 'Univision Puerto Rico';

UPDATE sources SET feed_type = 'scrape', feed_url = NULL, active = false
WHERE name = 'El Calce';

UPDATE sources SET feed_type = 'scrape', feed_url = NULL, active = false
WHERE name = 'Caribbean Business';

-- WAPA rate-limits (429)
UPDATE sources SET feed_type = 'scrape', feed_url = NULL, active = false
WHERE name = 'WAPA.TV';

UPDATE sources SET feed_type = 'scrape', feed_url = NULL, active = false
WHERE name = 'Noticentro (WAPA)';

-- Jay Fonseca: 500 error on /feed/
UPDATE sources SET active = false
WHERE name = 'Jay Fonseca';

-- ══════════════════════════════════════════════════════════════
-- PART 2: Fix existing sources — upgrade scrape → verified RSS
-- ══════════════════════════════════════════════════════════════

UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://www.primerahora.com/arc/outboundfeeds/rss/?outputType=xml',
    active = true
WHERE name = 'Primera Hora';

UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://www.metro.pr/arc/outboundfeeds/rss/?outputType=xml',
    active = true
WHERE name = 'Metro Puerto Rico';

-- Federal Register: fix to PR-specific search (Atom format)
UPDATE sources SET
    feed_url = 'https://www.federalregister.gov/documents/search.atom?conditions%5Bterm%5D=puerto+rico',
    region = 'Federal',
    active = true
WHERE name = 'Federal Register';

-- GovInfo: fix feed URL
UPDATE sources SET
    feed_url = 'https://www.govinfo.gov/rss/fr.xml',
    region = 'Federal',
    active = true
WHERE name = 'GovInfo';

-- ══════════════════════════════════════════════════════════════
-- PART 3: INSERT new PR media sources (verified working)
-- ══════════════════════════════════════════════════════════════

INSERT INTO sources (name, base_url, region, feed_type, feed_url, active) VALUES
('WIPR',
 'https://wipr.pr',
 'PR', 'rss',
 'https://wipr.pr/feed/',
 true),

('Hacia Donde Vamos',
 'https://haciadondevamos.com',
 'PR', 'rss',
 'https://anchor.fm/s/2ec2daac/podcast/rss',
 true),

('Puestos Pa''l Problema',
 'https://www.youtube.com/@PuestosPalProblema',
 'PR', 'rss',
 'https://anchor.fm/s/10d92fa8c/podcast/rss',
 true)
ON CONFLICT DO NOTHING;

-- ══════════════════════════════════════════════════════════════
-- PART 4: INSERT new PR Business & Ecosystem sources (verified working)
-- ══════════════════════════════════════════════════════════════

INSERT INTO sources (name, base_url, region, feed_type, feed_url, active) VALUES
('Colmena66',
 'https://colmena66.com',
 'PR', 'rss',
 'https://colmena66.com/feed/',
 true),

('Inprende',
 'https://inprende.com',
 'PR', 'rss',
 'https://inprende.com/feed/',
 true),

('PR Science Trust',
 'https://prsciencetrust.org',
 'PR', 'rss',
 'https://prsciencetrust.org/feed/',
 true),

('Foundation for Puerto Rico',
 'https://foundationforpuertorico.org',
 'PR', 'rss',
 'https://foundationforpuertorico.org/feed/',
 true)
ON CONFLICT DO NOTHING;

-- ══════════════════════════════════════════════════════════════
-- PART 5: INSERT US News sources (all verified working RSS)
-- ══════════════════════════════════════════════════════════════

INSERT INTO sources (name, base_url, region, feed_type, feed_url, active) VALUES
('ABC News',
 'https://abcnews.go.com',
 'US', 'rss',
 'http://feeds.abcnews.com/abcnews/topstories',
 true),

('Bloomberg',
 'https://www.bloomberg.com',
 'US', 'rss',
 'https://feeds.bloomberg.com/markets/news.rss',
 true),

('CBS News',
 'https://www.cbsnews.com',
 'US', 'rss',
 'https://www.cbsnews.com/latest/rss/main',
 true),

('CNN',
 'https://www.cnn.com',
 'US', 'rss',
 'http://rss.cnn.com/rss/cnn_topstories.rss',
 true),

('Forbes',
 'https://www.forbes.com',
 'US', 'rss',
 'https://www.forbes.com/innovation/feed2/',
 true),

('FOX News',
 'https://www.foxnews.com',
 'US', 'rss',
 'https://moxie.foxnews.com/google-publisher/us.xml',
 true),

('Harvard Business Review',
 'https://hbr.org',
 'US', 'rss',
 'http://feeds.harvardbusiness.org/harvardbusiness?format=xml',
 true),

('NBC News',
 'https://www.nbcnews.com',
 'US', 'rss',
 'http://feeds.nbcnews.com/nbcnews/public/news',
 true),

('TechCrunch',
 'https://techcrunch.com',
 'US', 'rss',
 'https://techcrunch.com/feed/',
 true),

('The New York Times',
 'https://www.nytimes.com',
 'US', 'rss',
 'http://rss.nytimes.com/services/xml/rss/nyt/HomePage.xml',
 true),

('The Rundown AI',
 'https://therundown.ai',
 'US', 'rss',
 'https://rss.beehiiv.com/feeds/2R3C6Bt5wj.xml',
 true),

('Wall Street Journal',
 'https://www.wsj.com',
 'US', 'rss',
 'https://feeds.a.dj.com/rss/RSSWorldNews.xml',
 true),

('Washington Post',
 'https://www.washingtonpost.com',
 'US', 'rss',
 'https://feeds.washingtonpost.com/rss/national',
 true),

('Y Combinator News',
 'https://news.ycombinator.com',
 'US', 'rss',
 'https://news.ycombinator.com/rss',
 true)
ON CONFLICT DO NOTHING;

-- Done: 14 US sources + 7 PR sources + fixes to existing sources
