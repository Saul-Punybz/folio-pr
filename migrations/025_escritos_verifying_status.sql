ALTER TABLE escritos
DROP CONSTRAINT IF EXISTS escritos_status_check;

ALTER TABLE escritos
ADD CONSTRAINT escritos_status_check
CHECK (status IN ('queued','planning','generating','scoring','verifying','done','failed','improving'));
