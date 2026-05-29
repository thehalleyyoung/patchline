-- Faulty migration: intended to backfill missing totals, but overwrites all issued invoices.
update invoices
set total_cents = 0
where status = 'issued';
