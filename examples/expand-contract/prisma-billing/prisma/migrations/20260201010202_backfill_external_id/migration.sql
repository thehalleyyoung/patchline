UPDATE invoices SET external_id = legacy_external_id WHERE external_id IS NULL;
