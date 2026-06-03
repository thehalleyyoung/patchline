# Database version semantics

Patchline's `db-semantics` command evaluates migration SQL against an explicit database engine and version instead of treating SQL as portable text. The current catalog covers PostgreSQL, MySQL, SQLite, SQL Server, Oracle, BigQuery, Snowflake, and ClickHouse with documented evidence for transactional DDL, implicit commits, atomic DDL, concurrent/online index behavior, online-schema-change adapters, replication-lag risk, query-plan regression preflight checks, instant or metadata-only column additions, create-or-replace replacement semantics, Time Travel recovery, partition-aware DDL, asynchronous mutations, and rollback feasibility for irreversible metadata changes.

The command emits deterministic JSON with the resolved profile, statement-level rules, proof obligations, risk counts, and a content hash:

```bash
go run ./cmd/patchline db-semantics \
  --engine postgres \
  --version 15 \
  --sql examples/db-version-semantics/semantics.sql \
  --out results/generated/db-version-semantics/postgres15.json \
  --json
```

The gate demonstrates version-specific behavior with real code:

- PostgreSQL 10 flags `ADD COLUMN ... DEFAULT` as a table rewrite, while PostgreSQL 11+ records metadata-only default semantics.
- MySQL 5.7 flags `ADD COLUMN` as copy/pre-instant risk, while MySQL 8.0.12+ recognizes eligible instant add-column semantics.
- `pt-online-schema-change`, `gh-ost`, PostgreSQL native concurrent indexes, Rails `strong_migrations`, and Django `AddIndexConcurrently` emit explicit adapter evidence and obligations.
- Query-plan regression preflight emits qualitative before/after representative workloads for index and column changes, while handing off to native `EXPLAIN`, `query-shape`, `index-coverage`, or `db-dry-run` evidence before trusting a safety or improvement claim.
- BigQuery and Snowflake distinguish `CREATE OR REPLACE TABLE` replacement hazards.
- ClickHouse marks `ALTER ... DELETE` as an asynchronous mutation requiring completion evidence.
- SQLite, SQL Server, and Oracle contribute connection-level FK, schema-lock, validation, and implicit-commit obligations.
- Rollback feasibility records whether the path is native transactional rollback, implicit-commit compensation, irreversible metadata restore, async mutation cleanup, DML before-image compensation, or snapshot-required bulk recovery.

Reproduce the evidence with:

```bash
make db-version-semantics-gate
make db-rollback-feasibility-gate
make query-plan-regression-gate
```
