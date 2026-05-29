# Current operational usefulness

Patchline is tailored for the problems platform teams are dealing with now: high-change databases, AI-assisted but human-owned codebases, noisy observability data, and strict PR controls. It stays non-AI by turning evidence into reproducible graphs and gates.

## Incident evidence ingest

Operational telemetry usually already exists in Datadog, OpenTelemetry collectors, deploy logs, migration runners, and logical decoding streams. Patchline's `ingest-evidence` command accepts a compact JSONL bridge format:

```bash
go run ./cmd/patchline adapt-evidence otlp examples/evidence/otlp-span-export.json --out /tmp/patchline-events.jsonl
go run ./cmd/patchline adapt-evidence datadog examples/evidence/datadog-span-export.json --out /tmp/patchline-events.jsonl
go run ./cmd/patchline adapt-evidence postgres examples/evidence/postgres-logical-decoding.json --out /tmp/patchline-events.jsonl
go run ./cmd/patchline adapt-evidence github examples/evidence/github-deployments.json --out /tmp/patchline-deploys.jsonl
go run ./cmd/patchline adapt-evidence migration-runner examples/evidence/migration-runner.json --out /tmp/patchline-migrations.jsonl
go run ./cmd/patchline ingest-evidence /tmp/patchline-events.jsonl --out /tmp/patchline-adapted-graph.json
go run ./cmd/patchline ingest-evidence examples/incidents/bad-migration.jsonl --json
go run ./cmd/patchline ingest-evidence examples/incidents/bad-migration.jsonl --out /tmp/patchline-graph.json
go run ./cmd/patchline explain report:monthly_revenue --graph /tmp/patchline-graph.json
```

Supported event types:

| Type | Purpose |
| --- | --- |
| `deploy` | Connect a commit and service to a deploy marker |
| `migration` | Connect a migration to the deploy that ran it |
| `trace` | Connect runtime trace evidence to a migration |
| `sql_mutation` | Connect a normalized query fingerprint to a trace |
| `row_mutation` | Connect a SQL mutation to a corrupted record |
| `derived_record` | Connect corrupted source records to downstream records |
| `derived_report` | Connect corrupted records to downstream reports |

The ingest path validates required fields, ignores unknown Datadog/OpenTelemetry-style fields, counts ignored fields, sorts entities and edges, and emits stable hashes:

| Hash | Meaning |
| --- | --- |
| `input_hash` | Sorted non-empty evidence lines |
| `entity_hash` | Canonical entity projection |
| `edge_hash` | Canonical edge projection |
| `graph_hash` | Versioned graph identity |

This makes observability evidence usable in repair review without forcing teams to abandon their current telemetry systems.

The native OTLP, Datadog, Postgres logical decoding, GitHub deployment/release, and migration-runner adapters intentionally require enough identifiers to emit graph-safe edges. SQL spans or row changes that cannot be tied to a migration are skipped with warnings, which keeps the generated JSONL directly ingestible and makes missing instrumentation visible in CI logs.

## Semantic contract audit

Patchline now has an executable semantic contract for current outputs:

```bash
go run ./cmd/patchline semantics-contract
go run ./cmd/patchline semantics-audit
go run ./cmd/patchline trace-reconstruct examples/incidents/bad-migration.jsonl
go run ./cmd/patchline trace-equivalence examples/incidents/bad-migration.jsonl examples/incidents/bad-migration.jsonl
go run ./cmd/patchline provenance certificate record:invoices/inv_1002 --evidence examples/incidents/bad-migration.jsonl
go run ./cmd/patchline provenance archive examples/incidents/bad-migration.jsonl examples/incidents/bad-migration.jsonl
go run ./cmd/patchline lint-repair examples/repairs/repair-bad-invoice-backfill.json --proof
go run ./cmd/patchline solver-obligations examples/repairs/repair-bad-invoice-backfill.json --invariants examples/invariants/billing-core.json
go run ./cmd/patchline symbolic-exec examples/repairs/repair-bad-invoice-backfill.json
go run ./cmd/patchline model-check-workflow examples/workflows/bad-migration-approved.json
go run ./cmd/patchline repair-semantics examples/repairs/repair-bad-invoice-backfill.json
go run ./cmd/patchline repair-semantics examples/repairs/repair-bad-invoice-backfill.json --store examples/snapshots/billing-bad-migration-before.json
go run ./cmd/patchline snapshot-drift examples/repairs/repair-bad-invoice-backfill.json examples/snapshots/billing-bad-migration-before.json examples/snapshots/billing-bad-migration-before.json
go run ./cmd/patchline schema-diff demos/billing/migrations/001_schema.sql examples/schemas/empty.json examples/schemas/billing-v1.json
go run ./cmd/patchline migration-semantics demos/billing/migrations/001_schema.sql examples/schemas/empty.json
go run ./cmd/patchline extract-sql examples/source-sql
go run ./cmd/patchline migration-outcomes examples/incidents/bad-migration.jsonl demos/billing/migrations/002_bad_backfill.sql --repair examples/repairs/repair-bad-invoice-backfill.json --policy examples/policies/review-required.json --benchmark examples/benchmarks/strict-migration-corpus.json --source-sql examples/source-sql
go run ./cmd/patchline effect-summary examples/repairs/repair-bad-invoice-backfill.json
go run ./cmd/patchline check-invariants examples/repairs/repair-bad-invoice-backfill.json examples/invariants/billing-core.json
go run ./cmd/patchline discover-invariants examples/repairs/repair-bad-invoice-backfill.json
```

The audit runs against the existing bad-migration incident evidence, repair manifest, replay report, replay-semantics report, invariant report, solver-obligation report, symbolic-execution report, workflow model-check report, generated SQL, transaction plan, migration analysis, schema semantics, source-code SQL inventory, migration outcome history, benchmark corpus, and ledger checkpoint. It reports facts, obligations, proof holes, counterexamples, and stable hashes, so historical data immediately exposes which claims are checked and which still need stronger proof machinery.

`trace-reconstruct` adds immediate value before deeper proof work: it shows whether imported evidence has exact, causal, temporal, inferred, absent, or conflicting support, normalizes event-time intervals, and produces a semantic projection hash that can be compared across Datadog/OTLP/Postgres/GitHub conversion paths.

`provenance certificate` adds the reviewer-facing artifact: minimal cause, smallest causal slice, semiring evidence summary, blast radius, stable hash, and missing-evidence holes. `provenance archive` gives a strict non-ML recurring-cause signal by clustering incident exports by canonical provenance shape. `lint-repair --proof` adds the repair-side reviewer artifact: Hoare view, weakest preconditions, frame conditions, refinement checks, counterexamples, and a proof hash. `solver-obligations` discharges the first bounded solver layer: scope implication and frame checks in a quantifier-free equality fragment, plus finite-store row-count and invariant-preservation checks. `symbolic-exec` explores bounded row paths with guard constraints and symbolic assignments, making it visible why a row is or is not touched before looking at concrete diffs. `model-check-workflow` enumerates bounded incident workflow states and checks temporal review properties, including failing fixtures for apply-before-approval counterexamples and missing-rollback proof holes. `repair-semantics` adds small-step replay states, pairwise independence/commutativity observations, bounded confluence, isolation hazards, compensating-action semantics for append-only external effects, and replayable counterexamples. `snapshot-drift` reruns repairs over imported historical row snapshots and fails when matched rows or row diffs drift. `schema-diff` and `migration-semantics` make migration effects reviewable as typed relational-signature transformations instead of raw SQL text. `extract-sql` finds SQL and ORM/query-builder data effects hidden in application code, which makes historical repair analysis useful even when incidents originate outside migration files. `migration-outcomes` links migrations to observed traces, SQL mutations, row/report damage, repairs, policy failures, benchmark hashes, and source-SQL hashes, then emits a reviewer-facing semantic changelog. `effect-summary` pairs concrete dry-run diffs with a monotone abstract effect summary, making bounded rows, changed columns, reversibility, idempotence, downstream impact, and proof holes visible in review. `check-invariants` and `discover-invariants` validate declared historical facts before/after replay and expose candidate invariants without auto-accepting them.

## CI gate for repair-analysis quality

Patchline now has a PR-oriented gate:

```bash
go run ./cmd/patchline ci-gate examples/benchmarks/strict-migration-corpus.json \
  --min-precision 0.95 \
  --min-recall 0.95
```

The gate fails when:

1. A benchmark case label does not match the analyzer prediction.
2. A pinned report hash changes.
3. Precision or recall falls below configured thresholds.

Exit codes are intentionally machine-readable:

| Code | Meaning |
| ---: | --- |
| 1 | Usage, parse, or configuration failure |
| 2 | Benchmark case or pinned hash failure |
| 3 | Precision or recall threshold failure |

When `GITHUB_ACTIONS=true`, the CLI writes a Markdown digest to `$GITHUB_STEP_SUMMARY` and emits `::error` annotations for failed cases. That gives reviewers immediate feedback on migration-risk regressions instead of burying the signal in logs.

## GitHub Actions example

```yaml
name: patchline

on:
  pull_request:

jobs:
  repair-safety:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: go test ./...
      - run: go run ./cmd/patchline ci-gate examples/benchmarks/strict-migration-corpus.json --min-precision 0.95 --min-recall 0.95
```

## Why this is useful now

| Audience | Immediate value |
| --- | --- |
| SRE/platform teams | Convert incident telemetry into explainable repair scope and damaged-entity lists |
| Datadog-like infra teams | Demonstrates deterministic adapters from span, deploy, release, and row-change exports into auditable graph evidence |
| Database reliability teams | Adds PR gates around risky migrations and analyzer regressions |
| Security/compliance teams | Preserves hashable inputs, outputs, policy decisions, and ledger checkpoints |
| Microsoft RiSE-style reviewers | Provides reproducible program-analysis benchmarks, executable semantics, bounded solver/model-check obligations, explicit labels, and stable hashes |

The next production adapter should add redaction and source-specific configuration files so teams can keep sensitive values out of evidence artifacts while preserving deterministic graph and benchmark semantics.
