ALTER TABLE quotations
ADD COLUMN status TEXT NOT NULL DEFAULT 'new';

CREATE INDEX quotations_status_idx ON quotations (status);
