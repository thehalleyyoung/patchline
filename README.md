# Patchline

Patchline is a deterministic repair-semantics workbench for production data incidents. Its core claim is that an outage, a migration, a row-change stream, a repair script, a backup decision, and a postmortem should not be separate narratives: they should compile into one typed transition system whose claims can be replayed, queried, proved, or refuted.

**Artifact-paper motivation.** Modern services routinely repair production state after bad migrations, unsafe deletes, divergent writes, failed backups, and stale derived outputs. The hard part is not writing one cleanup query; it is connecting the operational evidence, code/data transition, intended repair, rollback story, and future recurrence risk into a checkable object. Patchline makes that object explicit: public or local historical records become source-grounded observations; observations become typed traces and causal graphs; repairs become bounded state transformers; safety claims become Z3 obligations, replay hashes, policy decisions, and archive queries.

**Promise today:** given incident evidence, migration SQL, a repair manifest, policy rules, benchmark fixtures, and public historical sources, Patchline reconstructs causal provenance, classifies risky data changes, proves repair scope obligations with Z3, replays the repair over a bounded store, checks invariants/workflow properties, emits signed/hashable artifacts, indexes incidents for recurrence analysis, and validates counterfactual failure-avoidance signals on source-verified public incidents.

**No AI. No guesses.** Patchline is intentionally built from inspectable mechanisms: typed provenance, Z3-backed proof obligations, effect inference, invariant checks, replayable repair manifests, stable canonical output, source-manifest verification, and hash-chained audit records.

## Verify the current usefulness claim

The repo already has a reproducible validation path for the kind of cross-layer incident review a large observability or infrastructure company needs: it checks a historical-style bad migration fixture end to end, fetches pinned public migration files from an open-source migration platform to validate analyzer behavior against real SQL, and validates public incident counterfactuals against source phrases, linked public issue data, source-derived observations, and Patchline semantic signals.

Prerequisites: Go 1.22+, `curl`, `jq`, and Z3. This repository was validated with `Z3 version 4.15.8 - 64 bit`.

```bash
z3 --version
make verify-usefulness
```

That target runs the unit suite, strict CI gate, a SHA-verified public migration corpus benchmark, Z3-backed solver obligations, the semantic audit, the incident archive index, deterministic historical archive queries, the historical-failure suite, and public-source phrase checks. The public corpus is downloaded into `examples/public-corpus/downloads/` from pinned raw URLs and checked against `examples/public-corpus/sources.json`; the SQL files are not vendored. The GitLab 2017 case now verifies the postmortem plus linked public issue/API documents for backup monitoring, point-in-time recovery, hourly snapshots, backup-restore testing, staging migration rollback tooling, environment differentiation, and hard-delete policy gaps.

The expected default semantic audit result is `20` conforming artifacts and `0` counterexamples. The expected historical-failure result includes a primary-data destructive-operation case and a split-brain conflicting-write case, both backed by public postmortem source checks. If Z3 is missing, Patchline does not pretend to prove solver obligations: the solver report records the Z3 failure and downgrades those claims instead of using a handwritten SMT substitute.

## Why this is novel

Patchline's research claim is not "another migration linter" or "another incident tracker." The novel premise is a non-ML, proof-carrying bridge between operational telemetry, relational program semantics, repair execution, CI gates, and historical incident knowledge. Existing systems usually cover one slice: provenance, SQL equivalence, database testing, workflow model checking, repair synthesis, or incident management. Patchline makes those artifacts compose into a single reproducible evidence object: source-grounded observations plus a causal graph plus a repair transformer plus proof obligations plus replay hashes plus archive buckets. That is the unifying promise: Patchline evaluates production repair as a historical program-semantics problem, not as a bag of independent checks.

See [`docs/literature-positioning.md`](docs/literature-positioning.md) for the prior-art matrix and [`docs/usefulness-validation.md`](docs/usefulness-validation.md) for the validation protocol.

## Why this repo exists

Production data failures rarely stay inside one layer. A bad migration corrupts rows, a queue replay duplicates ledger entries, a deploy changes validation, and a dashboard quietly reports the wrong number. Patchline treats those failures as software-engineering incidents with causal evidence, not as one-off SQL cleanups.

The first scaffold in this repo focuses on a small but real core:

| Area | What exists now |
| --- | --- |
| Provenance graph | Typed entities and edges plus deterministic cause reports, minimal explanations, blast radius, incident-shape diffing, and causal certificates |
| Evidence ingestion | JSONL operational evidence ingest plus deterministic trace reconstruction with confidence and clock uncertainty |
| Repair manifests | JSON repair DSL with semantic validation, Hoare-style proof obligations, frame checks, and SQL refinement checks |
| Solver obligations | Z3-backed scope implication proofs plus finite-store frame, row-count, and invariant-preservation checks with explicit downgrade if Z3 is unavailable |
| Symbolic execution | Bounded repair-program row paths with guard constraints, symbolic assignments, and stuck-step hashes |
| Workflow model checking | Bounded ingest/explain/approve/dry-run/apply/verify/rollback/audit/archive state exploration with temporal properties, proof holes, and counterexample fixtures |
| CEGAR refinement | Counterexample/proof-hole guided reruns that refine coarse repair abstractions with invariant specs and incident workflow models |
| Incident archive | Deterministic archive index and historical queries over evidence, migrations, repair manifests, policies, and benchmark results, bucketed by semantic shape, broad updates, damaged-derived reports, rollback availability, and decisions |
| Historical failures | Public counterfactual suite that checks postmortems, linked public issue/API records, source-derived observations, destructive primary-data mutations, rollback gaps, damaged reports, and split-brain conflicting writes |
| Effect inference | A deterministic effect lattice and abstract interpreter over replay diffs |
| Migration analysis | SQL migration triage plus schema-state diffing, typed relational-signature semantics, and source-code SQL extraction |
| Replay sandbox | In-memory and imported-snapshot dry-run engine that emits stable row diffs, compensating-action semantics, and snapshot drift reports |
| Attestation checks | Executable checks for row diffs, operation effects, blast radius, downstream impact, hashes, plus Ed25519 signatures over semantic artifacts |
| Reproducibility artifacts | Benchmark-style manifests that pin dry-run hashes and ledger checkpoints |
| Strict benchmark suites | Frozen corpora with labels, pinned analyzer hashes, precision, and recall |
| CI gate | PR-friendly benchmark gate with precision/recall floors, GitHub Actions summaries, and annotations |
| Policy gates and bundles | Deterministic review policies plus proof-carrying incident bundle manifests for handoff |
| Audit ledger | Hash-chained repair ledger with checkpoint verification |
| Semantic contract | Hashable state/observation/repair contract plus conformance audit over historical artifacts |
| CLI | Commands for explanation, validation, dry-run replay, graph slicing, policy evaluation, bundles, benchmarks, and ledger verification |

See [`docs/rise-research-agenda.md`](docs/rise-research-agenda.md) for the research positioning.
See [`docs/current-operations.md`](docs/current-operations.md) for the current SRE and CI workflow positioning.
See [`docs/evidence-jsonl.md`](docs/evidence-jsonl.md) for the telemetry bridge format.
See [`docs/repair-manifests.md`](docs/repair-manifests.md) for migration, template, and lint tooling.
See [`docs/effect-lattice.md`](docs/effect-lattice.md) for the deterministic effect lattice and abstraction relation.
See [`docs/invariants.md`](docs/invariants.md) for invariant declarations, before/after checks, and candidate discovery.
See [`docs/solver-obligations.md`](docs/solver-obligations.md) for Z3-backed proof obligations.
See [`docs/symbolic-execution.md`](docs/symbolic-execution.md) for bounded repair-program path constraints.
See [`docs/workflow-model-checking.md`](docs/workflow-model-checking.md) for temporal incident workflow checks and proof holes.
See [`docs/refinement-and-attestations.md`](docs/refinement-and-attestations.md) for CEGAR-style refinement and signed artifact verification.
See [`docs/archive-index.md`](docs/archive-index.md) for semantic incident archives over historical evidence and repairs.
See [`docs/historical-failures.md`](docs/historical-failures.md) for public postmortem counterfactual validation.
See [`docs/replay-semantics.md`](docs/replay-semantics.md) for small-step traces, commutativity/confluence checks, and isolation hazards.
See [`docs/migration-analysis.md`](docs/migration-analysis.md) for generic and dialect-specific SQL migration analysis.
See [`docs/schema-semantics.md`](docs/schema-semantics.md) for schema diffs and relational-signature migration semantics.
See [`docs/source-sql-extraction.md`](docs/source-sql-extraction.md) for embedded SQL and ORM/query-builder extraction.
See [`docs/migration-outcomes.md`](docs/migration-outcomes.md) for migration outcome histories and semantic changelogs.
See [`docs/semantics.md`](docs/semantics.md) for the semantic contract, state model, observation model, and conformance audit.

## Quick start

```bash
go test ./...
go run ./cmd/patchline about
go run ./cmd/patchline semantics-contract
go run ./cmd/patchline semantics-audit
go run ./cmd/patchline trace-reconstruct examples/incidents/bad-migration.jsonl
go run ./cmd/patchline provenance certificate record:invoices/inv_1002 --evidence examples/incidents/bad-migration.jsonl
go run ./cmd/patchline explain record:invoices/inv_1002
go run ./cmd/patchline validate-repair examples/repairs/repair-bad-invoice-backfill.json
go run ./cmd/patchline lint-repair examples/repairs/repair-bad-invoice-backfill.json --proof
go run ./cmd/patchline solver-obligations examples/repairs/repair-bad-invoice-backfill.json --invariants examples/invariants/billing-core.json
go run ./cmd/patchline symbolic-exec examples/repairs/repair-bad-invoice-backfill.json
go run ./cmd/patchline model-check-workflow examples/workflows/bad-migration-approved.json
go run ./cmd/patchline cegar-refine examples/repairs/repair-bad-invoice-backfill.json --store examples/snapshots/billing-bad-migration-before.json --invariants examples/invariants/billing-core.json --workflow examples/workflows/bad-migration-approved.json
go run ./cmd/patchline archive-index examples/archive/bad-migration-corpus.json
go run ./cmd/patchline archive-query examples/archive/bad-migration-corpus.json --json
go run ./cmd/patchline historical-failures examples/historical-failures/suite.json --json
go run ./cmd/patchline attestation-keygen --json
go run ./cmd/patchline analyze-migration demos/billing/migrations/002_bad_backfill.sql
go run ./cmd/patchline analyze-migration examples/migrations/sqlserver-top-delete.sql --dialect sqlserver
go run ./cmd/patchline schema-diff demos/billing/migrations/001_schema.sql examples/schemas/empty.json examples/schemas/billing-v1.json
go run ./cmd/patchline migration-semantics demos/billing/migrations/001_schema.sql examples/schemas/empty.json
go run ./cmd/patchline extract-sql examples/source-sql
go run ./cmd/patchline migration-outcomes examples/incidents/bad-migration.jsonl demos/billing/migrations/002_bad_backfill.sql --repair examples/repairs/repair-bad-invoice-backfill.json --policy examples/policies/review-required.json --benchmark examples/benchmarks/strict-migration-corpus.json --source-sql examples/source-sql
go run ./cmd/patchline dry-run examples/repairs/repair-bad-invoice-backfill.json --json
go run ./cmd/patchline repair-semantics examples/repairs/repair-bad-invoice-backfill.json
go run ./cmd/patchline repair-semantics examples/repairs/repair-bad-invoice-backfill.json --store examples/snapshots/billing-bad-migration-before.json
go run ./cmd/patchline snapshot-drift examples/repairs/repair-bad-invoice-backfill.json examples/snapshots/billing-bad-migration-before.json examples/snapshots/billing-bad-migration-before.json
go run ./cmd/patchline effect-summary examples/repairs/repair-bad-invoice-backfill.json
go run ./cmd/patchline check-invariants examples/repairs/repair-bad-invoice-backfill.json examples/invariants/billing-core.json
go run ./cmd/patchline reproduce examples/reproduce/bad-migration-billing.json
go run ./cmd/patchline evaluate-policy examples/policies/review-required.json examples/repairs/repair-bad-invoice-backfill.json demos/billing/migrations/002_bad_backfill.sql
go run ./cmd/patchline benchmark-suite examples/benchmarks/strict-migration-corpus.json
go run ./cmd/patchline adapt-evidence otlp examples/evidence/otlp-span-export.json --out /tmp/patchline-events.jsonl
go run ./cmd/patchline adapt-evidence postgres examples/evidence/postgres-logical-decoding.json --out /tmp/patchline-events.jsonl
go run ./cmd/patchline ingest-evidence examples/incidents/bad-migration.jsonl
go run ./cmd/patchline ingest-evidence examples/incidents/bad-migration.jsonl --out /tmp/patchline-graph.json
go run ./cmd/patchline explain report:monthly_revenue --graph /tmp/patchline-graph.json
go run ./cmd/patchline ci-gate examples/benchmarks/strict-migration-corpus.json --min-precision 0.95 --min-recall 0.95
go run ./cmd/patchline ledger-verify --json
```

Or:

```bash
make test
make demo
```

## Example dry run

The included billing scenario models a faulty invoice backfill. Patchline can trace the bad row back to the migration and compute the deterministic repair diff:

```bash
go run ./cmd/patchline explain record:invoices/inv_1002
go run ./cmd/patchline analyze-migration demos/billing/migrations/002_bad_backfill.sql
go run ./cmd/patchline dry-run examples/repairs/repair-bad-invoice-backfill.json
go run ./cmd/patchline benchmark examples/reproduce/bad-migration-billing.json
go run ./cmd/patchline benchmark-suite examples/benchmarks/strict-migration-corpus.json
```

The benchmark artifact pins the expected dry-run hash and ledger checkpoint, then verifies invariant-style checks such as changed row values, max changed rows, operation effect, downstream impact, and scoped updates.

The benchmark suite adds a stricter corpus-level signal: every case has a human label and pinned migration-analysis hash, and the runner reports precision and recall. See [`docs/strict-benchmarking.md`](docs/strict-benchmarking.md) for the real-world validation protocol.

For current SRE/platform workflows, Patchline can also ingest incident evidence JSONL and run as a PR gate:

```bash
go run ./cmd/patchline adapt-evidence otlp examples/evidence/otlp-span-export.json --out /tmp/patchline-events.jsonl
go run ./cmd/patchline adapt-evidence github examples/evidence/github-deployments.json --out /tmp/patchline-deploys.jsonl
go run ./cmd/patchline adapt-evidence migration-runner examples/evidence/migration-runner.json --out /tmp/patchline-migrations.jsonl
go run ./cmd/patchline ingest-evidence /tmp/patchline-events.jsonl --out /tmp/patchline-adapted-graph.json
go run ./cmd/patchline ingest-evidence examples/incidents/bad-migration.jsonl --out /tmp/patchline-graph.json
go run ./cmd/patchline trace-reconstruct examples/incidents/bad-migration.jsonl
go run ./cmd/patchline trace-equivalence examples/incidents/bad-migration.jsonl examples/incidents/bad-migration.jsonl
go run ./cmd/patchline provenance cause record:invoices/inv_1002 --evidence examples/incidents/bad-migration.jsonl
go run ./cmd/patchline provenance blast record:invoices/inv_1002 --evidence examples/incidents/bad-migration.jsonl
go run ./cmd/patchline provenance diff examples/incidents/bad-migration.jsonl examples/incidents/bad-migration.jsonl
go run ./cmd/patchline provenance archive examples/incidents/bad-migration.jsonl examples/incidents/bad-migration.jsonl
go run ./cmd/patchline slice record:invoices/inv_1002 --graph /tmp/patchline-graph.json --json
go run ./cmd/patchline lint-repair examples/repairs/repair-bad-invoice-backfill.json
go run ./cmd/patchline lint-repair examples/repairs/repair-bad-invoice-backfill.json --proof --json
go run ./cmd/patchline generate-sql examples/repairs/repair-bad-invoice-backfill.json
go run ./cmd/patchline repair-semantics examples/repairs/repair-bad-invoice-backfill.json --json
go run ./cmd/patchline cegar-refine examples/repairs/repair-bad-invoice-backfill.json --store examples/snapshots/billing-bad-migration-before.json --invariants examples/invariants/billing-core.json --workflow examples/workflows/bad-migration-approved.json --json > /tmp/patchline-refinement.json
go run ./cmd/patchline archive-index examples/archive/bad-migration-corpus.json --json > /tmp/patchline-archive.json
go run ./cmd/patchline sign-artifact /tmp/patchline-refinement.json --subject cegar:billing-bad-migration --seed-hex "$PATCHLINE_ATTESTATION_SEED" --out /tmp/patchline-refinement.attestation.json
go run ./cmd/patchline verify-artifact /tmp/patchline-refinement.attestation.json --artifact /tmp/patchline-refinement.json
go run ./cmd/patchline effect-summary examples/repairs/repair-bad-invoice-backfill.json --json
go run ./cmd/patchline discover-invariants examples/repairs/repair-bad-invoice-backfill.json --json
go run ./cmd/patchline rollback-plan examples/repairs/repair-bad-invoice-backfill.json
go run ./cmd/patchline transaction-plan examples/repairs/repair-bad-invoice-backfill.json
go run ./cmd/patchline ci-gate examples/benchmarks/strict-migration-corpus.json --min-precision 0.95 --min-recall 0.95
```

The adapter path converts current span exports, Postgres logical decoding, GitHub deployment/release, and migration-runner JSON exports into Patchline evidence JSONL. Ingest intentionally accepts extra source-system fields while validating required Patchline evidence fields, so operational exports can be converted without losing auditability.

`trace-reconstruct` turns those same JSONL files into a typed trace projection with source confidence, clock confidence, normalized event-time intervals, and a semantic projection hash. `trace-equivalence` compares two imports by reconstructed projection instead of by raw line order or JSON field order.

The `provenance` subcommands turn historical traces into immediately reviewable artifacts: minimal causes, common ancestors, affected observations, semiring evidence summaries, smallest causal slices, differential provenance between incidents, recurring shape buckets, blast-radius summaries, and causal certificates with missing-evidence holes.

Repair manifests also have operational tooling:

```bash
go run ./cmd/patchline migrate-repair examples/repairs/legacy-v0-repair.json
go run ./cmd/patchline template-repair row-restore
go run ./cmd/patchline lint-repair examples/repairs/repair-bad-invoice-backfill.json --json
go run ./cmd/patchline lint-repair examples/repairs/repair-bad-invoice-backfill.json --proof --json
go run ./cmd/patchline generate-sql examples/repairs/repair-bad-invoice-backfill.json --json
go run ./cmd/patchline rollback-plan examples/repairs/repair-bad-invoice-backfill.json --json
go run ./cmd/patchline transaction-plan examples/repairs/repair-bad-invoice-backfill.json --json
```

The linter emits deterministic severity, code, reference, and remediation fields so repair review can be automated without making the manifest format loose. With `--proof`, it also emits a hashable Hoare-triple view, weakest preconditions, syntactic frame conditions, and SQL refinement checks, separating checked facts from assumed database obligations. SQL, rollback-plan, and transaction-plan generation are hashable and require the manifest to pass lint before producing executable statements, including scoped insert/delete repairs.

## Repository layout

```text
cmd/patchline/          CLI entrypoint
internal/provenance/   Typed causal graph and deterministic incident analysis
internal/evidence/     Operational evidence ingestion, adapters, and trace reconstruction
internal/effects/      Deterministic repair-effect inference
internal/migration/    SQL migration lexical analyzer and risk classifier
internal/repair/       Repair manifest parser and validator
internal/replay/       Dry-run state model and canonical reports
internal/attest/       Executable reproducibility and invariant checks
internal/refinement/   Counterexample-guided semantic refinement reports
internal/archive/      Historical incident archive indexes and semantic buckets
internal/historical/   Public postmortem counterfactual validation
internal/ledger/       Hash-chained audit ledger
internal/reproduce/    Benchmark/reproducibility runner
internal/bench/        Strict corpus benchmark runner
internal/gate/         CI threshold gate over benchmark suites
internal/policy/       Deterministic repair policy gates
internal/bundle/       Incident bundle manifests
internal/semantics/    Semantic contract and conformance audit artifacts
internal/demo/         Reproducible billing incident fixture
examples/              Incident and repair manifests
demos/billing/         SQL migration scenario
docs/                  Architecture, DSL, provenance, and RiSE research notes
```

## Status

This is a working deterministic core and demo harness, not yet the full production system. The next natural expansions are richer repair outcome histories, organization-local benchmark generation, redaction-preserving proof artifacts, reusable GitHub Action packaging, and a web incident cockpit.
