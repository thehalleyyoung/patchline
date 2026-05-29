CREATE INDEX CONCURRENTLY idx_invoices_status ON invoices(status);

ALTER TABLE invoices ADD COLUMN repaired_at timestamptz DEFAULT now();
