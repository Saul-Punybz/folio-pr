-- Folio: Political Intelligence System — Seed Sources
-- 24 news and government data sources across PR and Grants regions.
--
-- feed_type='rss'    — source has a known RSS/Atom feed
-- feed_type='scrape' — source requires web scraping (selectors configured later)

INSERT INTO sources (name, base_url, region, feed_type, feed_url, link_selector, title_selector, body_selector, date_selector) VALUES

-- ── Puerto Rico News Sources ─────────────────────────────────

-- 1. El Nuevo Dia (RSS)
('El Nuevo Dia',
 'https://www.elnuevodia.com',
 'PR', 'rss',
 'https://www.elnuevodia.com/arc/outboundfeeds/rss/?outputType=xml',
 '', '', '', ''),

-- 2. Primera Hora
('Primera Hora',
 'https://www.primerahora.com',
 'PR', 'scrape',
 NULL,
 '', '', '', ''),

-- 3. El Vocero
('El Vocero',
 'https://www.elvocero.com',
 'PR', 'scrape',
 NULL,
 '', '', '', ''),

-- 4. NotiCel
('NotiCel',
 'https://www.noticel.com',
 'PR', 'scrape',
 NULL,
 '', '', '', ''),

-- 5. Metro Puerto Rico
('Metro Puerto Rico',
 'https://www.metro.pr',
 'PR', 'scrape',
 NULL,
 '', '', '', ''),

-- 6. Noticentro (WAPA)
('Noticentro (WAPA)',
 'https://noticentro.tv',
 'PR', 'scrape',
 NULL,
 '', '', '', ''),

-- 7. WAPA.TV
('WAPA.TV',
 'https://www.wapa.tv/noticias',
 'PR', 'scrape',
 NULL,
 '', '', '', ''),

-- 8. Telemundo Puerto Rico
('Telemundo Puerto Rico',
 'https://www.telemundopr.com',
 'PR', 'scrape',
 NULL,
 '', '', '', ''),

-- 9. Univision Puerto Rico
('Univision Puerto Rico',
 'https://www.univision.com/local/puerto-rico-wlii',
 'PR', 'scrape',
 NULL,
 '', '', '', ''),

-- 10. TeleOnce
('TeleOnce',
 'https://teleonce.com',
 'PR', 'scrape',
 NULL,
 '', '', '', ''),

-- 11. Radio Isla 1320
('Radio Isla 1320',
 'https://radioisla.tv',
 'PR', 'scrape',
 NULL,
 '', '', '', ''),

-- 12. CPI (Centro de Periodismo Investigativo)
('CPI',
 'https://periodismoinvestigativo.com',
 'PR', 'scrape',
 NULL,
 '', '', '', ''),

-- 13. News is My Business
('News is My Business',
 'https://newsismybusiness.com',
 'PR', 'scrape',
 NULL,
 '', '', '', ''),

-- 14. Caribbean Business
('Caribbean Business',
 'https://caribbeanbusiness.com',
 'PR', 'scrape',
 NULL,
 '', '', '', ''),

-- 15. El Calce
('El Calce',
 'https://www.elcalce.com',
 'PR', 'scrape',
 NULL,
 '', '', '', ''),

-- 16. Es Noticia PR
('Es Noticia PR',
 'https://esnoticiapr.com',
 'PR', 'scrape',
 NULL,
 '', '', '', ''),

-- 17. Jay Fonseca
('Jay Fonseca',
 'https://jayfonseca.com',
 'PR', 'scrape',
 NULL,
 '', '', '', ''),

-- 18. Senado de Puerto Rico
('Senado de Puerto Rico',
 'https://senado.pr.gov',
 'PR', 'scrape',
 NULL,
 '', '', '', ''),

-- 19. Camara de Representantes
('Camara de Representantes',
 'https://tucamarapr.org',
 'PR', 'scrape',
 NULL,
 '', '', '', ''),

-- 20. La Fortaleza
('La Fortaleza',
 'https://www.fortaleza.pr.gov',
 'PR', 'scrape',
 NULL,
 '', '', '', ''),

-- ── Federal / Grants Sources ─────────────────────────────────

-- 21. Grants.gov (RSS)
('Grants.gov',
 'https://www.grants.gov',
 'Grants', 'rss',
 'https://www.grants.gov/rss/GG_NewOppByCategory.xml',
 '', '', '', ''),

-- 22. GAO Reports (RSS)
('GAO Reports',
 'https://www.gao.gov',
 'Grants', 'rss',
 'https://www.gao.gov/rss/reports.xml',
 '', '', '', ''),

-- 23. GovInfo (RSS)
('GovInfo',
 'https://www.govinfo.gov',
 'Grants', 'rss',
 'https://www.govinfo.gov/feeds',
 '', '', '', ''),

-- 24. Federal Register (RSS/Atom)
('Federal Register',
 'https://www.federalregister.gov',
 'Grants', 'rss',
 'https://www.federalregister.gov/documents/search.atom',
 '', '', '', '');
