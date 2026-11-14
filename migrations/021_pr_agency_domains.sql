-- PR Government agency domains for grant/RFP tracking
-- Only inserts domains that don't already exist

INSERT INTO crawl_domains (domain, label, category, max_depth, recrawl_hours, priority, active)
VALUES
  -- Centralized Portals
  ('subastas.pr.gov', 'Registro Unico de Subastas', 'government', 3, 24, 10, true),
  ('recovery.pr.gov', 'COR3 - Recuperacion', 'federal', 3, 48, 9, true),
  ('recuperacion.pr.gov', 'CDBG-DR Procurement', 'federal', 3, 48, 9, true),
  ('fondosfederales.pr.gov', 'Fondos Federales PR', 'federal', 3, 48, 9, true),

  -- Individual Agencies
  ('jp.pr.gov', 'Junta de Planificacion', 'government', 3, 72, 7, true),
  ('asg.pr.gov', 'Admin. Servicios Generales', 'government', 3, 48, 8, true),
  ('desarrollo.pr.gov', 'DDEC - Desarrollo Economico', 'government', 3, 48, 8, true),
  ('cdbg-dr.pr.gov', 'Vivienda CDBG-DR', 'government', 3, 48, 8, true),
  ('prits.pr.gov', 'PRITS - Tecnologia', 'government', 3, 72, 7, true),
  ('familia.pr.gov', 'Depto. Familia', 'government', 3, 72, 6, true),
  ('agricultura.pr.gov', 'Depto. Agricultura', 'government', 3, 72, 6, true),
  ('tourism.pr.gov', 'Compania de Turismo', 'government', 3, 72, 6, true),
  ('bde.pr.gov', 'Banco Desarrollo Economico', 'government', 3, 72, 7, true),
  ('energia.pr.gov', 'Negociado de Energia (PREB)', 'government', 3, 72, 6, true),
  ('smartisland.pr.gov', 'Smart Island - Broadband', 'government', 3, 72, 6, true),
  ('p3.pr.gov', 'Autoridad APP (P3)', 'government', 3, 72, 7, true),
  ('prpa.pr.gov', 'Autoridad de Puertos', 'government', 3, 72, 6, true),

  -- Utilities
  ('lumapr.com', 'LUMA Energy', 'government', 2, 72, 5, true),
  ('acueductos.pr.gov', 'AAA - Acueductos', 'government', 3, 72, 5, true),

  -- Third-party Aggregators (high value)
  ('puertoricobids.net', 'PR Bid Network', 'other', 2, 24, 8, true),
  ('puertorico.grantwatch.com', 'GrantWatch PR', 'other', 2, 48, 7, true)

ON CONFLICT (domain) DO NOTHING;
