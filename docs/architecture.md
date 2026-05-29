# Patchline architecture

Patchline is organized around one invariant: a repair is only safe if its cause, scope, execution, verification, and rollback path are explicit.

## Current components

```text
cmd/patchline
  |
  +-- internal/demo ---------- reproducible incident fixture
  +-- internal/provenance ---- typed causal graph
  +-- internal/evidence ------ operational evidence JSONL ingest
  +-- internal/repair -------- manifest parser and semantic validator
  +-- internal/effects ------- deterministic effect inference
  +-- internal/migration ----- lexical SQL migration analyzer
  +-- internal/replay -------- dry-run state model and canonical report
  +-- internal/attest -------- invariant and reproducibility checks
  +-- internal/ledger -------- hash-chained audit records
  +-- internal/reproduce ----- benchmark artifact runner
  +-- internal/bench --------- strict corpus benchmark runner
  +-- internal/gate ---------- CI threshold gate
```

## Data flow

1. Operational evidence is converted into deploy, trace, migration, SQL mutation, row mutation, and derivation events.
2. Evidence ingest builds a sorted provenance projection with stable entity, edge, input, and graph hashes.
3. An incident is scoped to graph entities such as a migration, trace, SQL mutation, or corrupted row.
4. A repair manifest declares preconditions, operations, postconditions, and rollback requirements.
5. The validator checks manifest shape, graph references, operation dependencies, and operation safety.
6. The effect analyzer classifies each operation as reversible, idempotent, destructive, replay, derived rebuild, or unknown.
7. The replay engine applies supported operations to a cloned state model and emits deterministic diffs.
8. The attestation layer checks expected row values, max changed rows, operation effects, downstream impact, scoped updates, and report hashes.
9. The reproducibility runner packages the repair, dry-run hash, invariant checks, and ledger checkpoint as a benchmark artifact.
10. Benchmark suites and CI gates prevent analyzer regressions in pull requests.
11. The ledger records planning, dry-run, approval, application, verification, and rollback events in a hash chain.

## Future production adapters

- Native OTLP receiver for traces and deploy markers.
- Datadog API importer for monitor alerts, deploy events, and spans.
- Postgres logical decoding adapter for row mutations.
- SQL fingerprinting and table/column extraction.
- Migration parsers for Rails, Prisma, Flyway, Liquibase, Alembic, Goose, and Django.
- CI/test adapter to link invariant failures to code versions.
- Web incident cockpit for graph, timeline, dry-run diff, and approval workflows.
