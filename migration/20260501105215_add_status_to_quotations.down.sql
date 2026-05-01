DROP INDEX IF EXISTS quotations_status_idx;

ALTER TABLE quotations
DROP COLUMN IF EXISTS status;
