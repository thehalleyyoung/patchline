#!/usr/bin/env bash
psql "$DATABASE_URL" <<SQL
UPDATE invoices
SET repair_marker = 'shell'
WHERE id = 'inv_1002';
SQL
