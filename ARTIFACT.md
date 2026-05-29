# Patchline Artifact Guide

Patchline's artifact claim is:

> Patchline converts historical production data-repair incidents into executable semantic artifacts that can replay repairs, prove scope and frame obligations, and detect recurrence across future migrations.

This guide is for artifact reviewers. It separates the implemented evaluator path from the larger roadmap in `NEWEST_PLAN.md`.

## Implemented in this artifact path

The current reviewer path is intentionally narrow and executable from a fresh checkout:

1. Validate the benchmark and ground-truth manifests.
2. Run the Go unit suite.
3. Execute the semantic pipeline over committed smoke fixtures.
4. Emit deterministic JSON outputs under `results/generated/`.
5. Run negative/limitation cases that show unsupported or insufficient-evidence outcomes are represented explicitly.
6. Compare study report hashes and executable benchmark-manifest outputs against frozen expected reports.
7. Optionally fetch pinned public OSS migrations and run the same study and phase-aware benchmark protocols against real migration SQL.
8. Run an offline public-incident benchmark over source-derived GitLab/GitHub observations and an explicit insufficient-evidence boundary.
9. Regenerate paper-facing corpus, detection/actionability, ablation, historical-counterfactual, and scale tables from the checked reports.

The smoke path uses only committed fixtures and existing CLI commands. It does not require network access. Public-data targets are explicit and verify pinned hashes before using downloaded files.

The benchmark protocol is time-realistic: each manifest case declares `available_at`, each ground-truth file declares its phase plus allowed/excluded inputs, and validation rejects any allowed input whose earliest availability is later than the case phase. This is the artifact guard against hindsight leakage in pre-deploy and during-repair claims.

## Setup

Prerequisites:

- Go 1.22+
- `make`
- `jq`
- Z3 on `PATH` for solver-backed claims

Z3 is expected for the strongest artifact claims. If Z3 is absent, Patchline records solver downgrade information rather than replacing it with hand-written SMT logic.

## Container path

The repository includes a Dockerfile and devcontainer definition:

```bash
docker build -t patchline-artifact .
docker run --rm patchline-artifact make artifact-smoke
```

The container installs Go, Z3, `jq`, `curl`, `git`, and `make`.

## Quick reviewer path

```bash
make artifact-smoke
```

Expected runtime: under 5 minutes on a laptop-class machine.

The target writes outputs to:

```text
results/generated/artifact-smoke/
```

It runs:

- `go test ./...`
- `artifact-ground-truth`
- command-presence checks for the artifact study commands
- `scripts/validate-ground-truth.sh`
- `scripts/check-artifact-targets.sh`
- `semantics-audit`
- `trace-reconstruct`
- `analyze-migration`
- `solver-obligations`
- `repair-semantics`
- `archive-query`
- `semantic-regressions`
- `benchmark-suite`

## Baselines, ablations, and scale

```bash
make artifact-studies-compare
make artifact-studies-public-compare
```

This target writes:

```text
results/generated/artifact-studies/
  baselines.json
  baselines.md
  ablations.json
  ablations.md
  scale.json
  scale.md
  public-migrations/
    baselines.json
    baselines.md
    ablations.json
    ablations.md
    scale.json
    scale.md
```

The reports are intentionally reviewer-facing rather than paper-scale. They make the current utility claim executable:

- `artifact-baselines` compares Patchline with transparent DDL-grep, normalized SQL-rule, and semantic-effects-without-evidence baselines and reports both detection metrics and actionability metrics.
- `artifact-ablations` separates migration-only, migration+policy, migration+policy+solver, migration+policy+solver+archive, and full artifact modes.
- `artifact-scale` records bytes, statements, high-risk statements, touched tables, hashes, and analyzer time for the committed strict corpus.
- `artifact-studies-public` repeats the study reports on the pinned Bytebase migration corpus after fetching and hashing the source files. This network-backed corpus measures public migration detection and structural actionability, not archive/proof gains for cases that do not declare archive or repair inputs.

The ablation report only counts solver-backed evidence for cases that declare repair/invariant inputs. This keeps the claim falsifiable: migration text alone can produce risk signals, but not repair proofs.

## Paper-facing result tables

```bash
make artifact-tables
```

This target writes:

```text
results/generated/artifact-tables/
  summary.json
  summary.md
```

The `artifact-tables` command derives five deterministic ICSE-style tables from existing checked inputs: executable corpora, detection/actionability, semantic-evidence ablation, historical public-derived counterfactuals, and scale. It uses frozen benchmark reports plus the strict and public migration study specs, records source hashes, states the public-postmortem-derived boundary, and excludes machine-dependent timing from stable table claims.

The compare targets summarize `baselines.json`, `ablations.json`, and `scale.json` into committed expected-hash manifests:

```text
benchmarks/expected/studies-strict.json
benchmarks/expected/studies-public-migrations.json
```

The study comparer checks stable report `hash` fields instead of byte-for-byte JSON. This intentionally ignores machine-dependent timing fields in scale reports while still failing on semantic drift, missing reports, unexpected JSON files, or corrupted stored hashes. Maintainers can refresh these manifests after a deliberate semantics change with:

```bash
make artifact-studies-refresh
```

## Executable benchmark manifests

```bash
make artifact-benchmark-compare
```

This target runs:

```bash
go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/smoke.json
go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/smoke.json --out results/generated/artifact-benchmark/smoke-report.json
go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/negative.json
go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/negative.json --out results/generated/artifact-benchmark/negative-report.json
go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/repair_cases.json
go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/repair_cases.json --out results/generated/artifact-benchmark/repair-cases-report.json
go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/semantic_regressions.json
go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/semantic_regressions.json --out results/generated/artifact-benchmark/semantic-regressions-report.json
go run ./cmd/patchline artifact-benchmark compare results/generated/artifact-benchmark/smoke-report.json benchmarks/expected/smoke-report.json
go run ./cmd/patchline artifact-benchmark compare results/generated/artifact-benchmark/negative-report.json benchmarks/expected/negative-report.json
go run ./cmd/patchline artifact-benchmark compare results/generated/artifact-benchmark/repair-cases-report.json benchmarks/expected/repair-cases-report.json
go run ./cmd/patchline artifact-benchmark compare results/generated/artifact-benchmark/semantic-regressions-report.json benchmarks/expected/semantic-regressions-report.json
```

The runner separates prediction from comparison. During prediction, Patchline may use the manifest fixture and the case's allowed/excluded input kinds, but it does not use the ground-truth expected label to decide the result. Ground truth is used to validate references, enforce phase guards and input availability, and compare the final outcome. The comparison command exits non-zero if a case result or report hash drifts.

The offline compare path includes four committed manifest families:

- `smoke.json`: one end-to-end sample across migration, incident, repair, and regression stages;
- `negative.json`: unsupported, insufficient-evidence, phase-leakage, replay-boundary, and safe-nonrecurrence controls;
- `repair_cases.json`: during-repair replay/proof cases with explicit repair plans, bounded stores, invariant specs, and a manual-rollback boundary;
- `semantic_regressions.json`: archive-only recurrence detection plus a scoped corrective non-recurrence case.

The real-OSS public migration path is explicit because it fetches pinned public sources:

```bash
make artifact-benchmark-public
```

That target downloads five Bytebase migrations at commit `47d2522552ce44271680424bf31a4cddd8a50ab1`, verifies each SHA-256 hash from `examples/public-corpus/sources.json`, runs `examples/benchmarks/public-bytebase-migration-corpus.json`, and compares the artifact benchmark report with `benchmarks/expected/public-migrations-report.json`.

The public-incident path is offline but source-derived:

```bash
make artifact-benchmark-public-incidents
```

It validates `benchmarks/manifests/public_incidents.json`, runs GitLab 2017 and GitHub 2018 public source-observation fixtures, and checks that a public summary with no transition/repair facts remains `insufficient_evidence`.

The public-derived repair path is also offline and source-grounded:

```bash
make artifact-benchmark-public-repairs
```

It validates `benchmarks/manifests/public_repairs.json`, runs a single Patchline-authored counterfactual repair manifest derived from the GitLab 2017 public postmortem and follow-up issues, and compares the report with `benchmarks/expected/public-repairs-report.json`. This path intentionally checks a boundary claim: with only a repair plan and public recovery-gap evidence, Patchline should say `cannot_prove` rather than inventing snapshot rollback proof.

The paired public-derived archive path is offline and source-grounded:

```bash
make artifact-benchmark-public-archive
```

It validates `benchmarks/manifests/public_archive.json`, runs `examples/archive/public-postmortem-derived-paired-archive.json`, and compares the report with `benchmarks/expected/public-archive-report.json`. The source observations are public, while the migration SQL, repair manifests, and replay stores are explicit Patchline reconstructions. The case demonstrates the archive distinction between a GitLab no-snapshot repair that remains `cannot_prove`, a scoped GitHub reconciliation that is `verified`, and a later broad `issues` transition flagged as a shared high-risk-table recurrence.

If a semantic change intentionally changes the benchmark outputs, refresh the frozen reports with:

```bash
make artifact-benchmark-refresh
```

This command rewrites `benchmarks/expected/*-report.{json,md}`, then reruns `make artifact-benchmark-compare`, `make artifact-benchmark-public`, `make artifact-benchmark-public-incidents`, `make artifact-benchmark-public-repairs`, and `make artifact-benchmark-public-archive`. Reviewers should use compare targets; refresh is a maintainer workflow for updating golden files after deliberate semantic changes.

## Canonical demo

```bash
make artifact-demo
```

Expected runtime: under 5 minutes.

The demo emits:

```text
results/generated/artifact-demo/
  trace.json
  migration.json
  solver.json
  repair.json
  archive.json
  regressions.json
  summary.json
  summary.md
```

The final summary includes a deterministic `artifact_bundle_hash` computed from the stage JSON files.

## Ground-truth validation

```bash
make artifact-ground-truth-check
```

This validates every file under `benchmarks/ground_truth/` and every manifest under `benchmarks/manifests/`.

The Make target runs both the first-class Go validator:

```bash
go run ./cmd/patchline artifact-ground-truth benchmarks --json
```

and the shell validator used as an independent packaging check.

The validator fails if:

- a case has no `case_id`, `case_type`, `phase`, expected result, or evidence;
- a pre-deploy case cites postmortem-only evidence;
- evidence lacks locator/rationale metadata;
- a manifest references a missing ground-truth file;
- a manifest case ID disagrees with its ground-truth case ID.

## Negative cases

```bash
make artifact-negative-cases
```

The current negative-case path validates explicit smoke labels for:

- unsupported SQL fragments;
- insufficient public evidence;
- phase-leakage protection;
- non-replayable repairs;
- safe non-recurrence.

These cases are part of the artifact, not merely prose limitations.

## Relationship to `make verify-usefulness`

`make verify-usefulness` remains the broader project validation target. It includes network-backed public corpus fetching and source checks. `make artifact-smoke` is the artifact-review entry point: it is smaller, committed-fixture-only, and intended to be stable without network access. `make artifact-studies-compare` is the offline evaluator-facing measurement and drift-check entry point for current baseline/ablation/scale evidence, `make artifact-studies-public-compare` is the explicit public-data study drift check, `make artifact-tables` is the paper-facing table regeneration path, `make artifact-benchmark-compare` is the offline golden-output check for the executable manifest protocol, `make artifact-benchmark-public` is the explicit public-data benchmark check, `make artifact-benchmark-public-incidents`, `make artifact-benchmark-public-repairs`, and `make artifact-benchmark-public-archive` are offline public-source-derived checks, and the refresh targets are maintainer paths for rewriting expected outputs after intentional semantic changes.

## Data provenance

Smoke fixtures are committed under `examples/` and benchmark labels are committed under `benchmarks/ground_truth/`. Public historical references are represented as source URLs and hashes/notes where appropriate; the smoke path does not download them.

Network-backed corpus validation remains available through:

```bash
make verify-usefulness
make public-corpus
```

## Roadmap not yet claimed by the smoke path

The following are planned in `NEWEST_PLAN.md` but are not yet claimed as fully implemented artifact results:

- large public migration benchmark with hundreds or thousands of statements;
- multi-incident public postmortem benchmark with precision/recall;
- full baseline comparison table over large public corpora;
- full ablation study over multi-family benchmark suites;
- scale and memory measurements over large corpora.

The smoke artifact path is designed so these can be added without changing the central claim.
