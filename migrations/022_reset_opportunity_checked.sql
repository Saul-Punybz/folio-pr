-- Reset opportunity_checked so pages get re-analyzed with improved text cleaning.
-- Also delete existing low-confidence opportunities to let them be re-extracted.
DELETE FROM opportunities WHERE confidence < 60;
UPDATE crawled_pages SET opportunity_checked = false WHERE opportunity_checked = true;
