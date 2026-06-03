# Online-schema-change adapters

Patchline recognizes online schema-change mechanisms as first-class database
semantics instead of treating `pt-online-schema-change`, `gh-ost`, native
concurrent indexes, or framework migration helpers as comments around ordinary
DDL.

The online-schema-change adapters gate is part of `db-semantics`: it now emits
an `online_schema_change` record for each detected
adapter. The record identifies the adapter, mechanism, table, shadow-table or
binlog dependencies, cutover requirements, manual rollback needs, evidence
references, and proof obligations. The lock simulation remains conservative:
online paths avoid long writer-blocking table locks, but still expose cutover,
metadata, or concurrent-index phase barriers.

The gate proves five real adapter families through the CLI:

- Percona `pt-online-schema-change` with trigger-backed shadow-table copying.
- GitHub `gh-ost` with binlog-driven ghost-table copying.
- PostgreSQL native `CREATE INDEX CONCURRENTLY`.
- Rails `strong_migrations` concurrent-index helpers.
- Django PostgreSQL `AddIndexConcurrently`.

Reproduce:

```bash
make online-schema-change-adapters-gate
```
