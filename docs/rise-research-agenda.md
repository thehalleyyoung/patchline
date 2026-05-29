# RiSE-oriented research agenda

Patchline is not affiliated with Microsoft. This document explains why the repo is meant to be interesting to software-engineering research groups such as Microsoft Research RiSE.

## Thesis

Production data repair is a software-engineering problem, not just a database operation. The repair should be derived from explicit causal evidence, checked against program/data invariants, replayed deterministically, and recorded in a tamper-evident artifact.

## Research hooks

### 1. Program analysis for repair effects

The first concrete artifacts are `internal/effects` and `internal/migration`. `internal/effects` is a small effect lattice plus an abstract interpreter for concrete replay diffs. `internal/migration` applies lexical SQL analysis to migration files, computes statement fingerprints, and flags broad or destructive changes.

The effect lattice classifies operations as:

- `reversible_update`
- `idempotent_update`
- `noop`
- `destructive`
- `replay`
- `derived_rebuild`
- `unknown`

The `effect-summary` command emits the abstraction/concretization relation, row bounds, changed-column sets, reversibility, idempotence, downstream impact, proof holes, and stable hashes. This is intentionally simple, but it creates a path toward deeper analysis:

- SQL AST classification.
- ORM write-path extraction.
- Migration effect inference.
- Schema constraint compatibility.
- Repair commutativity checks.
- Idempotence proofs for replay operations.

### 2. Typed causal provenance

`internal/provenance` models deploys, migrations, traces, SQL mutations, records, and reports as typed graph entities. This supports research questions such as:

- What evidence is sufficient to attribute a corrupted row to a code change?
- Which causal paths should be eligible for automated repair?
- How should uncertainty be represented without becoming probabilistic hand-waving?
- Can provenance paths become reproducible incident artifacts?

### 3. Deterministic replay and canonical outputs

`internal/replay` emits byte-stable dry-run reports. Tests assert the same manifest and state produce identical canonical bytes and hashes. This matters for reproducible research, differential testing, and repair review.

Future work:

- Snapshot isolation against real databases.
- Deterministic event replay for queues.
- Replay slicing from causal graph paths.
- Golden incident benchmarks.

`internal/bench` extends this idea to corpus-level evaluation: every migration case must pin a human label and an expected canonical report hash, and the runner emits precision and recall. This lets analyzer improvements be evaluated as research artifacts rather than anecdotes.

### 4. Verifiable transformations

Repair manifests encode preconditions, operations, postconditions, and rollback requirements. The current validator is small, but the intended direction is formal-ish:

- Preconditions as executable contracts.
- Postconditions as invariant suites.
- Operation dependency graphs with cycle detection.
- Proof-carrying repair bundles.
- Bounded blast-radius checks.
- Executable attestation suites for expected row diffs, operation effects, downstream impact, and canonical hashes.

### 5. Tamper-evident repair artifacts

`internal/ledger` records repair lifecycle events in a hash chain and verifies them against a checkpoint. This supports research into accountability for operational repair:

- Who approved the transformation?
- Which evidence justified it?
- Was the dry run identical to the applied repair?
- Can truncation or mutation be detected later?

## Benchmark direction

The repo now includes `examples/reproduce/bad-migration-billing.json`, a reproducibility artifact that pins:

- The repair manifest path.
- The expected dry-run report hash.
- The expected ledger checkpoint.
- The executable checks that define the benchmark outcome.

This makes the demo closer to a research benchmark than a screenshot-driven demo.

The repo also includes `examples/benchmarks/strict-migration-corpus.json`, a strict benchmark suite that pins migration-analysis hashes for both unsafe and safe cases. Empty hashes are rejected by the loader, so analyzer drift has to be reviewed explicitly.

Patchline should grow a suite of reproducible data-repair incidents:

| Scenario | Research value |
| --- | --- |
| Bad backfill migration | Migration analysis and row-level provenance |
| Duplicate queue replay | Idempotence and compensating repair |
| Schema drift report breakage | Static analysis across SQL and code |
| Timezone normalization bug | Data correction with bounded blast radius |
| Cache/index poisoning | Source-vs-derived repair planning |

Each benchmark should include seed data, faulty code/migration, observed telemetry, expected provenance paths, repair manifest, dry-run hash, and ledger checkpoint.

For real-world validation, Patchline should import public OSS migrations and incident patches under the protocol in [`strict-benchmarking.md`](strict-benchmarking.md): freeze the candidate list, label before scoring, preserve false positives/false negatives, and report precision/recall with pinned input and output hashes.
