create table invoices (
  id text primary key,
  customer_id text not null,
  status text not null,
  total_cents integer,
  expected_total_cents integer not null,
  repair_marker text
);

insert into invoices (id, customer_id, status, total_cents, expected_total_cents) values
  ('inv_1001', 'cus_001', 'issued', 1500, 1500),
  ('inv_1002', 'cus_002', 'issued', 4200, 4200);
