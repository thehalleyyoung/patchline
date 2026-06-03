ALTER TABLE accounts ADD COLUMN status text DEFAULT 'active';
CREATE INDEX idx_accounts_status ON accounts(status);
CREATE INDEX CONCURRENTLY idx_accounts_created_at ON accounts(created_at);
CREATE INDEX idx_accounts_email ON accounts(email) WITH (ONLINE=ON);
CREATE OR REPLACE TABLE analytics.daily AS SELECT * FROM analytics.stage;
ALTER TABLE events DELETE WHERE created_at < now();
PRAGMA foreign_keys = OFF;
