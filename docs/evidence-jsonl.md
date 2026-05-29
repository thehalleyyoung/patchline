# Evidence JSONL bridge format

Patchline evidence JSONL is the small, deterministic bridge between existing operational systems and the provenance graph. It is designed for adapters from Datadog, OpenTelemetry, deploy systems, migration runners, database logs, and repair tooling.

```bash
go run ./cmd/patchline adapt-evidence otlp examples/evidence/otlp-span-export.json --out /tmp/patchline-events.jsonl
go run ./cmd/patchline adapt-evidence postgres examples/evidence/postgres-logical-decoding.json --out /tmp/patchline-events.jsonl
go run ./cmd/patchline ingest-evidence examples/incidents/bad-migration.jsonl --out /tmp/patchline-graph.json
go run ./cmd/patchline trace-reconstruct examples/incidents/bad-migration.jsonl
go run ./cmd/patchline trace-equivalence examples/incidents/bad-migration.jsonl examples/incidents/bad-migration.jsonl
go run ./cmd/patchline explain report:monthly_revenue --graph /tmp/patchline-graph.json
go run ./cmd/patchline slice record:invoices/inv_1002 --graph /tmp/patchline-graph.json --json
```

## Format rules

1. Each non-empty line is one JSON object.
2. Every object must have a non-empty string `type`.
3. Required fields are validated per event type.
4. Trace-reconstruction fields such as `source_confidence`, `clock_confidence`, and event-time windows are supported on every event type.
5. Unknown fields are ignored and counted as `unknown_field_count`.
6. Events may appear in any order as long as all referenced entities are eventually defined.
7. Output entities, edges, damaged entities, and source type counts are sorted.
8. Hashes exclude wall-clock time, host paths, process IDs, and environment state.

Unknown fields are allowed because real Datadog/OpenTelemetry exports carry extra tags and attributes. Patchline keeps that behavior auditable by reporting the ignored field count.

## Trace reconstruction fields

Every event may include optional fields that turn graph ingestion into a typed historical trace projection:

| Field | Meaning |
| --- | --- |
| `source` | Source export name, for example `datadog`, `otlp`, `postgres`, or `migration-runner` |
| `source_confidence` | One of `exact`, `causal`, `temporal`, `inferred`, or `conflicting` |
| `clock_confidence` | One of `exact`, `temporal`, `inferred`, `absent`, or `conflicting` |
| `event_time`, `observed_at`, `timestamp`, `time` | Instant timestamp parsed as RFC3339/RFC3339Nano and normalized to UTC |
| `start_time`, `end_time`, `window_start`, `window_end` | Uncertainty interval endpoints parsed as RFC3339/RFC3339Nano and normalized to UTC |

When confidence is omitted, Patchline assigns conservative defaults: deploys, migrations, and row mutations are `exact`; runtime traces, SQL mutations, and derived outputs are `causal`; missing clocks are `absent`. Duplicate semantic facts with different times are marked `clock_confidence=conflicting`; duplicate facts from different sources are marked `source_confidence=conflicting`.

`trace-reconstruct` emits a `patchline.trace-projection/v1` artifact containing observations, confidence summaries, a normalized time range, raw-line hashes for audit, and a projection hash over typed semantics. `trace-equivalence` proves two imports reconstruct the same typed projection even when line order or JSON field order differs.

## Event types

### `deploy`

Connects a commit to a deploy marker and service.

Required fields:

| Field | Meaning |
| --- | --- |
| `id` | Stable deploy entity ID, for example `deploy:2026-05-29T12:00Z` |
| `commit` | Commit entity ID, for example `commit:8f3c2ab` |
| `service` | Service name, used to create `service:<name>` |

Example:

```json
{"type":"deploy","id":"deploy:2026-05-29T12:00Z","commit":"commit:8f3c2ab","service":"billing-api"}
```

### `migration`

Connects a migration to the deploy that ran it.

Required fields:

| Field | Meaning |
| --- | --- |
| `id` | Migration entity ID |
| `deploy` | Existing deploy entity ID |

Optional fields:

| Field | Meaning |
| --- | --- |
| `name` | Human-readable migration name |

Example:

```json
{"type":"migration","id":"migration:20260529_bad_invoice_backfill","deploy":"deploy:2026-05-29T12:00Z","name":"bad invoice total backfill"}
```

### `trace`

Connects runtime trace evidence to a migration.

Required fields:

| Field | Meaning |
| --- | --- |
| `id` | Trace entity ID |
| `migration` | Existing migration entity ID |

Example:

```json
{"type":"trace","id":"trace:7a44-billing-backfill","migration":"migration:20260529_bad_invoice_backfill"}
```

### `sql_mutation`

Connects a SQL statement or normalized query fingerprint to a trace.

Required fields:

| Field | Meaning |
| --- | --- |
| `id` | SQL mutation entity ID |
| `trace` | Existing trace entity ID |

Optional fields:

| Field | Meaning |
| --- | --- |
| `fingerprint` | Normalized query fingerprint |

Example:

```json
{"type":"sql_mutation","id":"sql:update_invoice_totals","trace":"trace:7a44-billing-backfill","fingerprint":"update invoices set total_cents = ? where status = ?"}
```

### `row_mutation`

Connects a SQL mutation to a damaged record.

Required fields:

| Field | Meaning |
| --- | --- |
| `record` | Record entity ID |
| `sql` | Existing SQL mutation entity ID |

Optional fields such as `before` and `after` are accepted for adapter convenience but are not yet used in graph hashing beyond the input hash.

Example:

```json
{"type":"row_mutation","record":"record:invoices/inv_1002","sql":"sql:update_invoice_totals","before":{"total_cents":"4200"},"after":{"total_cents":"0"}}
```

### `derived_record`

Connects a damaged source entity to a downstream record.

Required fields:

| Field | Meaning |
| --- | --- |
| `from` | Existing source entity ID |
| `to` | Downstream record entity ID |

Example:

```json
{"type":"derived_record","from":"record:invoices/inv_1002","to":"record:ledger_entries/le_777"}
```

### `derived_report`

Connects a damaged source entity to a downstream report.

Required fields:

| Field | Meaning |
| --- | --- |
| `from` | Existing source entity ID |
| `to` | Downstream report entity ID |

Example:

```json
{"type":"derived_report","from":"record:ledger_entries/le_777","to":"report:monthly_revenue"}
```

## Hashes

`ingest-evidence` emits four stable hashes:

| Hash | Input |
| --- | --- |
| `input_hash` | Sorted non-empty input lines |
| `entity_hash` | Canonical sorted entity projection |
| `edge_hash` | Canonical sorted edge projection |
| `graph_hash` | Versioned composition of `entity_hash` and `edge_hash` |

The graph projection written by `--out` contains `version`, `hash`, `entities`, and `edges`. It can be fed back into graph commands:

```bash
go run ./cmd/patchline explain report:monthly_revenue --graph /tmp/patchline-graph.json
```

## Adapter guidance

Adapters should do the smallest possible conversion:

| Source | Suggested mapping |
| --- | --- |
| Datadog deploy events | `deploy` |
| GitHub Deployments or releases | `deploy` |
| OpenTelemetry spans | `trace` and `sql_mutation` |
| Migration runner logs | `migration` |
| Postgres logical decoding | `row_mutation` |
| Data pipeline lineage | `derived_record` and `derived_report` |

The adapter should preserve source-specific fields as extra JSON fields when useful. Patchline will ignore them for graph construction, count them, and keep the original lines covered by `input_hash`.

Patchline includes native adapters for current operational exports:

```bash
go run ./cmd/patchline adapt-evidence otlp examples/evidence/otlp-span-export.json --out /tmp/patchline-events.jsonl
go run ./cmd/patchline adapt-evidence datadog examples/evidence/datadog-span-export.json --out /tmp/patchline-events.jsonl
go run ./cmd/patchline adapt-evidence postgres examples/evidence/postgres-logical-decoding.json --out /tmp/patchline-events.jsonl
go run ./cmd/patchline adapt-evidence github examples/evidence/github-deployments.json --out /tmp/patchline-deploys.jsonl
go run ./cmd/patchline adapt-evidence migration-runner examples/evidence/migration-runner.json --out /tmp/patchline-migrations.jsonl
go run ./cmd/patchline ingest-evidence /tmp/patchline-events.jsonl --out /tmp/patchline-graph.json
```

The OTLP adapter reads `resourceSpans[].scopeSpans[].spans[]` and `instrumentationLibrarySpans[]`. The Datadog adapter reads span objects with `trace_id`/`span_id` or `traceId`/`spanId`, plus deploy events with tags. Both adapters emit:

| Source field | Patchline output |
| --- | --- |
| `patchline.migration_id`, `db.migration.id`, or `migration.id` | `trace.migration` |
| `db.statement`, `db.query`, or `sql.query` | `sql_mutation.fingerprint` |
| `patchline.record_id` or `db.record.id` | `row_mutation.record` |
| Datadog `service`, `git.commit.sha`, `patchline.deploy_id` tags | `deploy` |

SQL spans without a migration identifier are skipped with a warning instead of producing dangling graph edges. This keeps adapter output directly ingestible while making missing instrumentation visible.

The Postgres adapter accepts wal2json-style logical decoding payloads with top-level Patchline metadata:

| Source field | Patchline output |
| --- | --- |
| `patchline_migration_id` | `migration`, `trace.migration` |
| `patchline_deploy_id`, `commit`, `service` | `deploy` and `migration.deploy` |
| `change[].kind/schema/table` | `sql_mutation.fingerprint` |
| `change[].oldkeys` or `columnnames`/`columnvalues` | `row_mutation.record` |

The GitHub adapter imports deployment and release exports as `deploy` evidence using deployment `sha`/`ref` or release `target_commitish`. The migration-runner adapter accepts normalized Rails/Django/Flyway/Liquibase/Goose-style JSON with `deploy_id`, `commit`, `service`, and `migrations[]`, emitting `deploy`, `migration`, optional `trace`, and optional `sql_mutation` events.
