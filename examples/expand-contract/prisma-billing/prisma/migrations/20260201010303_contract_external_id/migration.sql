ALTER TABLE invoices ALTER COLUMN external_id SET NOT NULL;
ALTER TABLE invoices DROP COLUMN legacy_external_id;
