# Provenance model

Patchline uses typed entities and edges instead of a generic graph. The type system is deliberately small in the first scaffold so causal paths are inspectable.

## Entity kinds

| Kind | Meaning |
| --- | --- |
| `service` | Running application or worker |
| `commit` | Source version |
| `deploy` | Runtime rollout of a commit/config |
| `migration` | Schema or data migration |
| `trace` | Runtime trace or span aggregate |
| `sql_mutation` | Normalized mutation fingerprint |
| `record` | Durable row/document/event |
| `job_run` | Background job execution |
| `queue_event` | Message or event delivery |
| `report` | Derived report, dashboard, cache, or index |
| `repair` | Applied repair artifact |

## Edge kinds

| Edge | Meaning |
| --- | --- |
| `deployed_commit` | Commit was deployed |
| `executed` | Deploy/job executed a migration or task |
| `caused` | Runtime event caused another event |
| `mutated` | Query or operation changed a record |
| `derived_into` | Record fed a derived record/report/cache |
| `observed` | Trace/log/metric observed an entity |
| `repaired` | Repair changed an entity |

## Evidence levels

Edges carry evidence levels:

- `exact`: shared transaction ID, migration ID, commit SHA, row mutation metadata.
- `strong`: shared trace ID, job run, event ID, or query fingerprint.
- `medium`: bounded correlation with supporting metadata.
- `weak`: temporal correlation only.

Patchline's CLI examples require at least strong evidence for causal explanation. Weak evidence should inform exploration, not automated repair.

## Deterministic cause queries

Patchline exposes Datalog-style provenance queries over any ingested incident graph:

```bash
go run ./cmd/patchline provenance cause record:invoices/inv_1002 --evidence examples/incidents/bad-migration.jsonl
go run ./cmd/patchline provenance minimal record:invoices/inv_1002 --evidence examples/incidents/bad-migration.jsonl
go run ./cmd/patchline provenance blast record:invoices/inv_1002 --evidence examples/incidents/bad-migration.jsonl
go run ./cmd/patchline provenance certificate record:invoices/inv_1002 --evidence examples/incidents/bad-migration.jsonl
```

The cause report computes:

| Field | Meaning |
| --- | --- |
| `minimal_causes` | Closest sufficient causal entities above the evidence threshold, usually SQL mutation, migration, deploy, or commit nodes |
| `all_cause_candidates` | Every causal candidate reachable by backward provenance search |
| `common_ancestors` | Entities shared by all queried affected starts |
| `affected_observations` | Records, reports, traces, and services reachable forward from the minimal causes |
| `repair_lineage` | Repair edges already connected to the graph |
| `semiring` | Evidence algebra summary and conflicts |
| `minimal_explanation` | Smallest deterministic union of shortest evidence paths justifying the causal claim |
| `blast_radius` | Affected tables, records, reports, services, and kind counts |

The evidence semiring uses deterministic path product and fact join:

- Path combination is the minimum evidence rank along the path.
- Fact join keeps the strongest support unless duplicate facts disagree, which is reported as `conflicting`.
- Supported values are `exact`, `strong`, `weak`, `absent`, `conflicting`, and `redacted`.

This is executable rather than aspirational: the bad-migration fixture yields a causal certificate with a stable certificate hash and an explicit missing-evidence hole when the minimal explanation stops at a SQL mutation rather than proving the SQL mutation's upstream trace cause.

## Differential and recurring provenance

Historical comparison and archive mining are available without ML:

```bash
go run ./cmd/patchline provenance diff examples/incidents/bad-migration.jsonl examples/incidents/bad-migration.jsonl
go run ./cmd/patchline provenance archive examples/incidents/bad-migration.jsonl examples/incidents/bad-migration.jsonl
```

`diff` compares canonical incident shapes and emits shared, left-only, and right-only causal structure. `archive` clusters incidents by canonical trace-shape hash so repeated migration, SQL, table, or derived-report failure patterns can be counted deterministically over historical exports.
