ALTER TABLE accounts ADD COLUMN status text DEFAULT 'active';
CREATE INDEX idx_accounts_status ON accounts(status);
CREATE OR REPLACE TABLE analytics.daily AS SELECT * FROM analytics.stage;
ALTER TABLE events DELETE WHERE created_at < now();
PRAGMA foreign_keys = OFF;
