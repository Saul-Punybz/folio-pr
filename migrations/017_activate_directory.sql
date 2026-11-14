-- Migration 017: Activate full media directory, convert scrape→RSS, add US region to sources
-- Activates all sources from the PR & US media directory

-- ── 1. Fix sources region CHECK to allow US ─────────────────────
ALTER TABLE sources DROP CONSTRAINT IF EXISTS sources_region_check;
ALTER TABLE sources ADD CONSTRAINT sources_region_check
    CHECK (region IN ('PR', 'US', 'Grants', 'Federal', 'Local', 'Global'));

-- ── 2. Convert PR scrape sources to RSS where feeds exist ───────

-- Telemundo Puerto Rico
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://www.telemundopr.com/noticias/?rss=y',
    active = true
WHERE lower(name) LIKE '%telemundo%';

-- WAPA TV
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://www.wapa.tv/feed/',
    active = true
WHERE lower(name) LIKE '%wapa%';

-- WIPR
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://wipr.pr/feed/',
    active = true
WHERE lower(name) LIKE '%wipr%';

-- El Vocero
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://www.elvocero.com/search/?f=rss&t=article&l=50&s=start_time&sd=desc',
    active = true
WHERE lower(name) LIKE '%vocero%';

-- El Calce
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://www.elcalce.com/feed/',
    active = true
WHERE lower(name) LIKE '%calce%';

-- NotiCel
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://www.noticel.com/feed/',
    active = true
WHERE lower(name) LIKE '%noticel%';

-- Platea
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://platea.pr/feed/',
    active = true
WHERE lower(name) LIKE '%platea%';

-- NotiUno 630
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://www.notiuno.com/feed/',
    active = true
WHERE lower(name) LIKE '%notiuno%';

-- WKAQ 580
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://wkaq580.com/feed/',
    active = true
WHERE lower(name) LIKE '%wkaq%';

-- CPI (Centro de Periodismo Investigativo)
UPDATE sources SET active = true
WHERE lower(name) LIKE '%cpi%';

-- News is My Business
UPDATE sources SET active = true
WHERE lower(name) LIKE '%news is my business%';

-- Es Noticia PR
UPDATE sources SET active = true
WHERE lower(name) LIKE '%es noticia%';

-- Jay Fonseca
UPDATE sources SET active = true
WHERE lower(name) LIKE '%jay fonseca%';

-- Caribbean Business
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://caribbeanbusiness.com/feed/',
    active = true
WHERE lower(name) LIKE '%caribbean business%';

-- ── 3. PR Business & Ecosystem — convert to RSS where possible ──

-- Colmena66
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://colmena66.com/feed/',
    active = true
WHERE lower(name) LIKE '%colmena%';

-- Grupo Guayacán
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://grupoguayacan.com/feed/',
    active = true
WHERE lower(name) LIKE '%guayac%';

-- Parallel18
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://www.parallel18.com/blog-feed.xml',
    active = true
WHERE lower(name) LIKE '%parallel%';

-- Centro para Emprendedores
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://centroparaemprendedores.org/feed/',
    active = true
WHERE lower(name) LIKE '%emprendedores%';

-- Inprende
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://inprende.com/feed/',
    active = true
WHERE lower(name) LIKE '%inprende%';

-- Echar Pa'lante
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://echarpalante.org/feed/',
    active = true
WHERE lower(name) LIKE '%echar%';

-- YES Media
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://yesmedia.pr/feed/',
    active = true
WHERE lower(name) LIKE '%yes media%';

-- GEW Puerto Rico
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://gewpuertorico.com/feed/',
    active = true
WHERE lower(name) LIKE '%gew%';

-- PR Science Trust
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://prsciencetrust.org/feed/',
    active = true
WHERE lower(name) LIKE '%science%trust%' OR lower(name) LIKE '%science trust%';

-- RICCE
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://ricce.org/feed/',
    active = true
WHERE lower(name) LIKE '%ricce%';

-- ── 4. PR Impact & Social organizations ─────────────────────────

-- Foundation for Puerto Rico
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://foundationforpuertorico.org/feed/',
    active = true
WHERE lower(name) LIKE '%foundation for%puerto%';

-- Invest Puerto Rico
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://www.investpr.org/feed/',
    active = true
WHERE lower(name) LIKE '%invest%puerto%';

-- ConPRometidos
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://comprometidos.com/feed/',
    active = true
WHERE lower(name) LIKE '%comprometidos%';

-- IDJ
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://juventud.pr.gov/feed/',
    active = true
WHERE lower(name) LIKE '%idj%' OR lower(name) LIKE '%juventud%';

-- Mentes en Acción
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://mentesenaccion.org/feed/',
    active = true
WHERE lower(name) LIKE '%mentes%';

-- Centro para Puerto Rico (Sila Calderón)
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://centroparapuertorico.org/feed/',
    active = true
WHERE lower(name) LIKE '%centro para puerto%';

-- Fondos Unidos
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://fondosunidos.org/feed/',
    active = true
WHERE lower(name) LIKE '%fondos%';

-- Centro Sor Isolina Ferré
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://centrosorisolinaferreaguadilla.org/feed/',
    active = true
WHERE lower(name) LIKE '%isolina%';

-- Friends of Puerto Rico
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://friendsofpuertorico.org/feed/',
    active = true
WHERE lower(name) LIKE '%friends%';

-- Vitrina Solidaria
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://vitrinasolidaria.org/feed/',
    active = true
WHERE lower(name) LIKE '%vitrina%';

-- Carlos Cobián
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://carloscobianmedia.com/feed/',
    active = true
WHERE lower(name) LIKE '%cobi%';

-- Discover Puerto Rico
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://www.discoverpuertorico.com/blog/feed',
    active = true
WHERE lower(name) LIKE '%discover%';

-- ── 5. Activate all Federal/Grants sources ──────────────────────

UPDATE sources SET active = true
WHERE lower(name) LIKE '%federal register%';

UPDATE sources SET active = true
WHERE lower(name) LIKE '%govinfo%';

UPDATE sources SET active = true
WHERE lower(name) LIKE '%gao%';

-- ── 6. Activate government scrape sources ───────────────────────

-- La Fortaleza
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://www.fortaleza.pr.gov/feed/',
    active = true
WHERE lower(name) LIKE '%fortaleza%';

-- Senado de Puerto Rico
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://senado.pr.gov/feed/',
    active = true
WHERE lower(name) LIKE '%senado%';

-- Camara de Representantes
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://www.tucamarapr.org/feed/',
    active = true
WHERE lower(name) LIKE '%camara%';

-- Univision Puerto Rico
UPDATE sources SET feed_type = 'rss',
    feed_url = 'https://www.univision.com/feeds/feed-noticias-puerto-rico',
    active = true
WHERE lower(name) LIKE '%univision%';

-- ── 7. Ensure all existing active sources stay active ───────────
-- (Already active: El Nuevo Dia, Primera Hora, Metro PR, Puestos Pa'l Problema,
--  Hacia Donde Vamos, Radio Isla, TeleOnce, all US sources)

-- Make absolutely sure all US sources are active
UPDATE sources SET active = true WHERE region = 'US';

-- Done
