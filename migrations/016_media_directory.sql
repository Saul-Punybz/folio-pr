-- 016: Expand region constraint and add PR + US media directory sources.

-- Allow US and Global regions.
ALTER TABLE sources DROP CONSTRAINT IF EXISTS sources_region_check;
ALTER TABLE sources ADD CONSTRAINT sources_region_check
  CHECK (region = ANY (ARRAY['PR', 'US', 'Global', 'Grants', 'Federal', 'Local']));
