-- Corrective repair represented by examples/repairs/repair-bad-invoice-backfill.json.
update invoices
set
  total_cents = expected_total_cents,
  repair_marker = 'inc_bad_migration_001'
where id = 'inv_1002'
  and total_cents = 0;
