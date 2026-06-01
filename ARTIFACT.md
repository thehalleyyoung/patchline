# Patchline Artifact Guide

Patchline's current-use claim is:

> Patchline can be pointed at existing repos, GitHub projects, migration directories, source trees, telemetry/deploy exports, JSON logs, incident docs, and repair artifacts to produce deterministic problem/cause/repair findings before a team adopts Patchline-specific labels or schemas.

The artifact validation claim is:

> Patchline converts historical production data-repair incidents into executable semantic artifacts that can replay repairs, prove scope and frame obligations, and detect recurrence across future migrations.

This guide is for artifact reviewers. Start with `patchline intake <path>` or `patchline intake --github owner/repo --subpath path` for arbitrary current data; the rest of this guide separates the implemented evaluator path from longer-range validation work that is not yet claimed by the artifact path.

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
9. Run a dedicated phase/input availability check over every benchmark manifest.
10. Regenerate paper-facing corpus, detection/actionability, ablation, historical-counterfactual, and scale tables from the checked reports.
11. Emit a defined-subtask comparison report that states the public and strict-corpus baseline/ablation wins and fails if those win conditions drift.
12. Audit the benchmark-selection protocol against a candidate-pool ledger, required manifest coverage, evidence-kind coverage, and reviewer/maintainer command separation.
13. Emit a canonical demo bundle that includes local repair proof/replay evidence, public-derived archive recurrence evidence, benchmark comparison output, and a per-file SHA-256 manifest.

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
- `phase-check` on the smoke manifest
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

- `artifact-baselines` compares Patchline with transparent raw-SQL DDL-grep, normalized SQL-rule, migration-guardrail-linter baselines, plus a clearly labeled Patchline effects-only ablation, and reports both detection metrics and actionability metrics.
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

The `artifact-tables` command derives five deterministic ICSE-style tables from existing checked inputs: executable corpora, detection/actionability, semantic-evidence ablation, historical public-derived counterfactuals, and scale. It uses regenerated benchmark reports only after their hashes match the frozen expected reports, records both actual and expected source hashes, states the public-postmortem-derived boundary, and excludes machine-dependent timing from stable table claims.

## Complete experiment-number ledger

```bash
make artifact-numbers
```

This target writes:

```text
results/generated/artifact-numbers/
  summary.json
  summary.md
```

The `artifact-numbers` command gathers every reportable number from the checked strict/public study reports and regenerated benchmark reports, then requires each generated benchmark hash to match the frozen expected report. Its JSON includes per-case baseline decisions, ablation decisions, scale rows, benchmark case outcomes, actual/expected report hashes, and input hashes; the Markdown file is a compact paper-facing index over the same hashed ledger.

The compare targets summarize `baselines.json`, `ablations.json`, and `scale.json` into committed expected-hash manifests:

```text
benchmarks/expected/studies-strict.json
benchmarks/expected/studies-public-migrations.json
```

The study comparer checks stable report `hash` fields instead of byte-for-byte JSON. This intentionally ignores machine-dependent timing fields in scale reports while still failing on semantic drift, missing reports, unexpected JSON files, or corrupted stored hashes. Maintainers can refresh these manifests after a deliberate semantics change with:

```bash
PATCHLINE_ACCEPT_EXPECTED_REFRESH=1 make artifact-studies-refresh
```

## Defined-subtask comparison report

```bash
make artifact-subtasks
```

This target writes:

```text
results/generated/artifact-subtasks/
  summary.json
  summary.md
```

The `artifact-subtasks` command consumes the checked experiment-number ledger and makes the current comparative claim executable: Patchline must beat raw-SQL DDL grep and normalized SQL rules on the public Bytebase migration-risk subtask, beat or enrich the guardrail/effects-only comparators, and carry proof/archive evidence beyond detector-style baselines on the strict repair-review subtask. If any required margin disappears, the command exits non-zero instead of leaving a stale README claim.

## Corpus-selection audit

```bash
make artifact-corpus-audit
```

This target writes:

```text
results/generated/artifact-corpus-audit/
  summary.json
  summary.md
```

The `artifact-corpus-audit` command makes benchmark selection auditable rather than prose-only. It hashes `benchmarks/corpus_protocol.json`, checks that every committed benchmark manifest appears in the protocol, verifies every included manifest case has a candidate-pool ledger entry, reports result and boundary coverage, checks required evidence kinds for each manifest family, and rejects reviewer-mode commands that would refresh golden files.

## Artifact-wide provenance receipt

```bash
make artifact-provenance
```

This target writes:

```text
results/generated/artifact-provenance/
  summary.json
  summary.md
```

The `artifact-provenance` command is the reviewer-facing reproducibility receipt for the generated artifact. It hashes benchmark manifests, ground-truth files, frozen expected outputs, public-corpus manifests, and generated reviewer reports; independently re-hashes manifest-derived cached public corpus files against `examples/public-corpus/sources.json` rather than trusting paths recorded by the fetch report; checks that the `artifact-demo` bundle manifest exactly matches the stage outputs on disk; and reruns the experiment-number ledger validation so stale benchmark outputs cannot be silently reported.

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

That target downloads five Bytebase migrations at commit `47d2522552ce44271680424bf31a4cddd8a50ab1` from the single source manifest `examples/public-corpus/sources.json`, verifies each SHA-256 hash, writes `results/generated/public-corpus/fetch-report.json`, runs `examples/benchmarks/public-bytebase-migration-corpus.json`, and compares the artifact benchmark report with `benchmarks/expected/public-migrations-report.json`. For network-free reruns with a prepopulated cache, set `PATCHLINE_PUBLIC_CORPUS_OFFLINE=1`; the fetch step then accepts only hash-valid cached files and fails on missing/corrupt inputs.

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
PATCHLINE_ACCEPT_EXPECTED_REFRESH=1 make artifact-benchmark-refresh
```

This command rewrites `benchmarks/expected/*-report.{json,md}` only after the explicit `PATCHLINE_ACCEPT_EXPECTED_REFRESH=1` guard is set. Reviewers should use compare targets; refresh is a maintainer workflow for updating golden files after deliberate semantic changes, and compare commands should be run separately after the expected-output diff is reviewed.

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
  public-archive-phase-check.json
  public-archive-index.json
  public-archive-query.json
  public-regressions.json
  public-archive-benchmark-validate.json
  public-archive-benchmark-run.json
  public-archive-benchmark-report.json
  public-archive-benchmark-compare.json
  bundle-manifest.json
  summary.json
  summary.md
```

The demo now combines a committed local billing-repair fixture with an offline public-postmortem-derived archive case. The final summary includes a deterministic `artifact_bundle_hash` computed from `bundle-manifest.json`, which records per-file SHA-256 hashes for every stage JSON file except the summary and manifest themselves. The current public archive slice reports one source-derived recurrence relation and checks the public archive benchmark against the frozen expected hash before including it in the bundle.

## Ground-truth validation

```bash
make artifact-ground-truth-check
make phase-check
```

This validates every file under `benchmarks/ground_truth/` and every manifest under `benchmarks/manifests/`. `make phase-check` additionally runs the first-class `patchline phase-check <manifest.json>` reviewer command across every committed manifest and prints each case's declared phase and input kind before failing on any availability violation.

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

`make verify-usefulness` remains the broader project validation target. It includes network-backed public corpus fetching and live source checks, so it is intentionally separate from `make artifact-full`. `make artifact-full` is the reviewer artifact path and does not include maintainer refreshes or live source validation. `make artifact-smoke` is the smallest artifact-review entry point: it is committed-fixture-only and intended to be stable without network access. `make phase-check` is the direct no-hindsight-leakage command path over committed manifests. `make artifact-studies-compare` is the offline evaluator-facing measurement and drift-check entry point for current baseline/ablation/scale evidence, `make artifact-studies-public-compare` is the explicit public-data study drift check, `make artifact-tables` is the paper-facing table regeneration path, `make artifact-numbers` is the exhaustive reportable-number ledger path, `make artifact-subtasks` is the explicit baseline/ablation win-condition report, `make artifact-corpus-audit` is the executable benchmark-selection/candidate-ledger audit, `make artifact-provenance` is the artifact-wide reproducibility receipt, `make artifact-benchmark-compare` is the offline golden-output check for the executable manifest protocol, `make artifact-benchmark-public` is the explicit public-data benchmark check, `make artifact-benchmark-public-incidents`, `make artifact-benchmark-public-repairs`, and `make artifact-benchmark-public-archive` are offline public-source-derived checks, and the refresh targets are guarded maintainer paths for rewriting expected outputs after intentional semantic changes.

## Data provenance

Smoke fixtures are committed under `examples/` and benchmark labels are committed under `benchmarks/ground_truth/`. Public historical references are represented as source URLs and hashes/notes where appropriate; the smoke path does not download them.

Network-backed corpus validation remains available through:

```bash
make verify-usefulness
make public-corpus
```

## Roadmap not yet claimed by the smoke path

The following are useful extensions but are not yet claimed as fully implemented artifact results:

- large public migration benchmark with hundreds or thousands of statements;
- multi-incident public postmortem benchmark with precision/recall;
- full baseline comparison table over large public corpora;
- full ablation study over multi-family benchmark suites;
- scale and memory measurements over large corpora.

The smoke artifact path is designed so these can be added without changing the central claim.
