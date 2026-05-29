# Patchline semantic core

This document identifies the small semantic core that the artifact paper should ask reviewers to inspect. Everything else in the repo is supporting infrastructure.

## State model

| State | Meaning | Example artifact |
| --- | --- | --- |
| Source state | Primary records before or during an incident. | `examples/snapshots/billing-bad-migration-before.json` |
| Derived state | Reports, ledgers, caches, or downstream outputs derived from source records. | `report:monthly_revenue` in archive fixtures |
| Damaged state | A state reached by an unsafe or under-scoped transition. | bad invoice backfill fixture |
| Repaired state | A state reached by an explicit repair manifest or compensating transition. | `repair-bad-invoice-backfill.json` |
| Archive state | Ordered historical memory of incidents, repairs, policy outcomes, and recurrence candidates. | `examples/archive/bad-migration-corpus.json` |

## Transition model

| Transition | Description | Existing command |
| --- | --- | --- |
| Evidence normalization | Convert logs/source observations into typed Patchline events. | `ingest-evidence`, `trace-reconstruct` |
| Migration transition | Classify SQL/data changes and schema effects. | `analyze-migration`, `migration-semantics` |
| Repair transition | Interpret explicit repair operations over a bounded store. | `repair-semantics`, `dry-run` |
| Proof transition | Emit and check scope/frame/invariant obligations. | `solver-obligations` |
| Replay transition | Execute repair semantics and produce stable row-diff hashes. | `dry-run`, `repair-outcomes` |
| Archive update | Index incidents and make historical queries deterministic. | `archive-index`, `archive-query` |
| Regression transition | Compare later incidents against prior semantic shapes. | `semantic-regressions` |

## Obligations

Patchline separates proved, checked, assumed, unsupported, and refuted claims.

| Obligation | Purpose | Backing |
| --- | --- | --- |
| Scope | The repair touches the intended rows/entities. | Z3 where expressible; bounded checks otherwise |
| Frame | The repair does not mutate unrelated rows/entities. | Z3 finite-store obligations and replay diffs |
| Row-count | The repair stays within declared blast-radius bounds. | solver/checker report |
| Invariant preservation | Declared invariants hold after repair. | invariant checks and solver obligations |
| Replay determinism | Re-running the repair over the bounded store yields stable hashes. | replay hash |
| Rollback availability | A compensating or rollback story exists. | repair outcome/archive fields |
| Recurrence review | Historical damaged shapes/tables/reports do not recur silently. | archive semantic regression query |

## Z3 mapping

Patchline uses Z3 through the solver-obligation path. The artifact should treat Z3-backed results differently from deterministic but non-solver checks.

| Solver-backed area | Current command | If Z3 is unavailable |
| --- | --- | --- |
| Scope implication | `solver-obligations` | report records downgrade/failure |
| Frame checks over bounded store | `solver-obligations` | report records downgrade/failure |
| Row-count checks | `solver-obligations` | report records downgrade/failure |
| Invariant preservation | `solver-obligations --invariants ...` | report records downgrade/failure |

Archive recurrence, source phrase checks, and migration heuristics are deterministic semantic checks, but they are not Z3 proofs.

## Implementation map

| Semantic concept | Go package/file | CLI command | Representative tests |
| --- | --- | --- | --- |
| Trace projection | `internal/evidence/trace.go` | `trace-reconstruct` | `internal/evidence/trace_test.go` |
| Provenance graph | `internal/provenance` | `provenance`, `explain`, `slice` | `internal/provenance/*_test.go` |
| Migration transition | `internal/migration` | `analyze-migration`, `migration-semantics` | `internal/migration/*_test.go` |
| Repair transition | `internal/repair`, `internal/replay` | `validate-repair`, `repair-semantics`, `dry-run` | `internal/repair/*_test.go`, `internal/replay/*_test.go` |
| Proof obligation | `internal/solver` | `solver-obligations` | `internal/solver/solver_test.go` |
| Semantic contract | `internal/semantics` | `semantics-contract`, `semantics-audit` | `internal/semantics/*_test.go` |
| Archive memory | `internal/archive` | `archive-index`, `archive-query`, `repair-outcomes` | `internal/archive/archive_test.go` |
| Regression memory | `internal/archive` | `semantic-regressions` | `internal/archive/archive_test.go` |
| Historical source checks | `internal/historical` | `historical-failures` | `internal/historical/historical_test.go` |

## JSON envelope status

Some commands already emit stable hashes and artifact-specific versions. A uniform `artifact_kind`, `artifact_version`, `artifact_hash`, and `inputs_hash` envelope is a planned hardening step, not yet a universal contract. The current artifact path therefore validates command outputs by existing stable fields and files rather than claiming a universal envelope.
