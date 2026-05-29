# NEWEST_PLAN: Making Patchline Artifact-Paper Ready

This plan turns the ICSE-style review critique into an implementation roadmap for making Patchline credible as the focus of a software engineering artifact paper. The north star is to move Patchline from a rich semantics-first prototype into a reproducible artifact with a single claim, public data, ground truth, baselines, ablations, and reviewer-friendly execution paths.

## Current State Snapshot (2026-05-29)

Patchline now has the artifact-review scaffold and several executable evaluator paths in place:

- `README.md`, `ARTIFACT.md`, `docs/paper-claim.md`, `docs/semantic-core.md`, `docs/semantic-pipeline.md`, and `docs/literature-positioning.md` frame the repository around the historical repair-semantics artifact claim.
- `benchmarks/LABELING.md`, `benchmarks/manifests/smoke.json`, `benchmarks/manifests/negative.json`, `benchmarks/manifests/public_migrations.json`, `benchmarks/manifests/public_incidents.json`, and `benchmarks/ground_truth/**` provide phase-aware, source-grounded labels.
- `patchline artifact-ground-truth`, `artifact-baselines`, `artifact-ablations`, `artifact-scale`, and `artifact-benchmark validate/run/compare` are implemented.
- `make artifact-studies` remains the offline strict-corpus study path, while `make artifact-studies-public` repeats baseline/ablation/scale reports on the pinned public Bytebase migration corpus.
- `make artifact-benchmark-compare` executes smoke and negative manifests, writes deterministic reports under `results/generated/artifact-benchmark/`, and compares them against committed golden reports in `benchmarks/expected/`.
- `make artifact-benchmark-public` fetches five pinned Bytebase migrations, verifies source hashes, runs the legacy benchmark-suite label/hash check, and compares the phase-aware artifact benchmark to a committed golden report.
- `make artifact-benchmark-public-incidents` runs an offline public-incident corpus over GitLab 2017 and GitHub 2018 source-derived observations plus a too-thin-public-summary boundary, comparing against a committed golden report.
- `make artifact-benchmark-refresh` is the explicit maintainer path for regenerating committed golden reports after deliberate semantic changes.
- `make artifact-full && make verify-usefulness` has passed after the executable benchmark runner, golden-refresh workflow, and public migration corpus were added.
- `100_STEPS.md`, `PLAN.md`, and `NEW_PLAN.md` are untracked/ignored; this file remains the tracked implementation roadmap.

The latest refinement expands artifact studies beyond one strict-corpus baseline: Patchline now compares against DDL-grep, normalized SQL-rule, and effects-without-evidence baselines, and it has an explicit network-backed public migration study target. The next large step should add pinned expected hashes for study reports and expand the public corpora rather than adding isolated demos.

## 0. Target Artifact Claim

Patchline should be organized around one memorable claim:

> Patchline converts historical production data-repair incidents into executable semantic artifacts that can replay repairs, prove scope and frame obligations, and detect recurrence across future migrations.

Everything in the repo should support this claim. Features that do not directly support it should either be reframed as supporting infrastructure or moved out of the primary artifact path.

### What this claim commits us to prove

1. Historical incidents can be represented as executable semantic artifacts, not just prose summaries.
2. Patchline can replay or model the relevant transition/repair behavior.
3. Patchline can emit proof obligations for scope, frame, rollback, replay, and recurrence properties.
4. Patchline uses Z3 for solver-backed obligations rather than hand-rolled SMT reasoning.
5. Patchline can compare future or later artifacts against historical semantic memory.
6. Patchline provides utility on public, reproducible data with externally checkable ground truth.

### What the artifact should not overclaim

Patchline should not claim to automatically understand arbitrary production systems, reconstruct unavailable private evidence, or prove that every incident would have been prevented. It should make phase-aware claims:

- **Pre-deploy:** risky transitions can be flagged before execution when the migration, schema, policy, and historical archive are available.
- **During repair:** repair plans can be replayed and checked against declared invariants and rollback constraints.
- **Postmortem/archive:** incidents can become reusable semantic memory for future recurrence detection.

## 1. Reviewer Gap Matrix

| Reviewer concern | Current risk | Required repo response | Done when |
| --- | --- | --- | --- |
| Single artifact claim | Repo feels broad: provenance, replay, repair, solver, archive, policy, CI, benchmarking | Make all README, docs, examples, and commands converge on the historical repair semantics claim | README, `ARTIFACT.md`, and `docs/paper-claim.md` all state the same claim and map commands to it |
| Real benchmark suite | Smoke/negative manifests plus first public migration/incident corpora are executable; corpus size is still small | Expand public benchmark datasets with machine-readable manifests, labels, expected outputs, and scale summaries | `make artifact-benchmark-public`, `make artifact-benchmark-public-incidents`, and `make artifact-studies-public` cover public corpora |
| Ground truth | Core protocol exists and validates | Store labels, evidence URLs, source snippets/hashes, labeling rules, and expected classifications | Every benchmark case has `ground_truth/*.json` with label provenance |
| Baselines | DDL-grep, normalized SQL-rule, and effects-without-evidence baselines run on strict and public migration corpora | Add pinned expected hashes and larger public corpus coverage | `make artifact-baselines` and `make artifact-baselines-public` emit comparative tables |
| Ablations | Strict and public migration study targets exist; public corpus intentionally measures only migration detection/actionability unless inputs declare proof/archive links | Add pinned expected hashes and public cases with repair/archive inputs | `make artifact-ablations` and `make artifact-ablations-public` show added signal/actionability per component |
| Time realism | Phase-aware ground truth and benchmark runner enforce input availability | Add `available_at` phase annotations and enforce phase-limited evaluation | Pre-deploy claims only consume pre-deploy evidence |
| Packaging | Docker/devcontainer, smoke/full targets, and artifact docs exist | Add Docker/devcontainer, `ARTIFACT.md`, smoke/full targets, cached data option | Fresh checkout can run smoke path under 5 minutes |
| Scale | `artifact-scale` and `artifact-scale-public` emit corpus measurements; corpus size is still small | Add larger corpus-scale benchmark and report runtime/memory/query latency | study targets emit benchmark JSON and Markdown tables for strict and public corpora |
| Trusted semantic core | `docs/semantic-core.md` exists and is linked from README | Define a small explicit core model and connect it to code and CLI commands | `docs/semantic-core.md` maps model terms to implementation files and tests |
| Negative cases | Five executable negative controls are in the benchmark path | Add unsupported/ambiguous cases and explicit failure modes | `make artifact-negative-cases` and `make artifact-benchmark-compare` show boundary outcomes |
| Literature novelty | `docs/literature-positioning.md` exists | Add literature-positioning doc and paper-intro material | `docs/literature-positioning.md` contrasts Patchline with provenance, migration linting, repair, incident tools |
| Killer demo | `make artifact-demo` exists; expected-output refresh still manual | Add one end-to-end demo over named public data | `make artifact-demo` emits a paper-ready bundle hash and result summary |

## 2. Workstream A: One Coherent Artifact Story

### Goal

Make the repository read as one system rather than a collection of useful tools. The unifying object is the **Patchline semantic artifact**:

```text
Evidence -> Trace -> Transition -> Repair -> Proof -> Replay -> Archive -> Regression
```

### Tasks

1. Create `docs/paper-claim.md`.
   - State the core artifact claim.
   - Define what Patchline means by:
     - evidence,
     - trace,
     - transition,
     - repair,
     - proof obligation,
     - replay,
     - archive,
     - semantic regression.
   - Include a “what is not claimed” section.

2. Revise `README.md` around the claim.
   - Opening paragraph should state the artifact claim directly.
   - Move broad feature lists below the semantic pipeline.
   - Keep instant usability, but make every command map to one pipeline stage.
   - Add a table:

     | Stage | Command | Artifact emitted | Why it matters |
     | --- | --- | --- | --- |
     | Evidence | `source-sql` / `historical-*` | source-grounded facts | public ground truth |
     | Trace | `trace-reconstruct` | trace hash | reproducible event semantics |
     | Transition | `analyze-migration` | migration risk report | pre-deploy detection |
     | Repair | `repair-semantics` | repair step trace | operational repair model |
     | Proof | `solver-obligations` | Z3-backed obligations | non-handwavy constraints |
     | Replay | `replay` / `repair-outcomes` | replay/verification hash | repair correctness evidence |
     | Archive | `archive-query` | corpus query result | historical memory |
     | Regression | `semantic-regressions` | recurrence invariants | future prevention |

3. Add a top-level `ARTIFACT.md`.
   - Audience: artifact evaluator.
   - Include:
     - claims,
     - setup,
     - smoke path,
     - full path,
     - expected runtime,
     - hardware assumptions,
     - data provenance,
     - exact commands for each paper claim,
     - troubleshooting.

4. Add one diagram source.
   - Prefer a plain Mermaid block in Markdown so it is reviewable in GitHub.
   - File: `docs/semantic-pipeline.md`.
   - The diagram should show data flow and phase boundaries.

### Acceptance criteria

- A reviewer can describe Patchline in one sentence after reading the first page of the README.
- No primary demo is merely “a command that prints something”; every demo says what artifact it emits and what claim it supports.
- The README does not use company-specific names in the core pitch.

## 3. Workstream B: Public Benchmark Suite

### Goal

Create a benchmark suite that demonstrates utility on public, reproducible data with ground truth.

### Directory structure

```text
benchmarks/
  README.md
  manifests/
    public_migrations.json
    public_incidents.json
    repair_cases.json
    semantic_regressions.json
  ground_truth/
    migrations/
    incidents/
    repairs/
    regressions/
  expected/
    patchline/
    baselines/
    ablations/
  cache/
    README.md
```

`benchmarks/cache/` should not require committing large downloaded corpora unless necessary. It should support:

- fetch-on-demand mode,
- pinned URL/SHA mode,
- optional local cache mode,
- small committed fixtures for smoke tests.

### Dataset 1: Public OSS migration corpus

Purpose: evaluate risky migration detection and semantic enrichment.

Candidate sources:

- Public GitHub repositories with SQL migration directories.
- Bytebase public SQL review examples if license-compatible.
- Rails/Django/Laravel migration histories in public repos.
- GitLab CE historical migration files, if license-compatible and clone size manageable.

Manifest schema:

```json
{
  "dataset_id": "public-oss-migrations-v1",
  "version": "1",
  "source": {
    "kind": "github",
    "repo": "owner/repo",
    "ref": "commit-sha",
    "path": "db/migrate"
  },
  "cases": [
    {
      "case_id": "repo-commit-file-statement",
      "file": "db/migrate/...",
      "statement_index": 0,
      "available_at": "pre_deploy",
      "labels": {
        "broad_update": true,
        "destructive": false,
        "requires_backfill_guard": true,
        "expected_risk": "high"
      },
      "label_evidence": [
        {
          "kind": "source_rule",
          "rule": "UPDATE without WHERE"
        }
      ]
    }
  ]
}
```

Initial target sizes:

- Smoke: 10 migration files.
- Paper-minimum: 250 migration statements.
- Strong artifact: 1,000+ migration statements from at least 5 repositories.

### Dataset 2: Public historical incident corpus

Purpose: evaluate counterfactual gates and phase-aware historical claims.

Candidate sources:

- GitLab 2017 database incident public postmortem.
- Public postmortems involving bad migrations, destructive updates, bad deploys, cache/data divergence, rollback failures.
- Cloudflare, GitHub, Sentry, Shopify, Stripe, or other public engineering incident writeups when license/terms allow source citation.

Manifest schema:

```json
{
  "dataset_id": "public-incidents-v1",
  "cases": [
    {
      "incident_id": "gitlab-2017-db",
      "source_url": "...",
      "source_hash": "sha256:...",
      "available_at": "postmortem",
      "failure_class": [
        "destructive-primary-data-transition",
        "backup-restore-gap"
      ],
      "counterfactual_claims": [
        {
          "claim_id": "pre-deploy-broad-mutation",
          "phase": "pre_deploy",
          "allowed_inputs": ["migration_text", "declared_policy"],
          "expected_patchline_result": "gate_fail",
          "ground_truth_basis": "source paragraph or archived snippet hash"
        }
      ]
    }
  ]
}
```

Initial target sizes:

- Smoke: 2 incidents.
- Paper-minimum: 10 incidents.
- Strong artifact: 25+ incidents with at least 3 failure classes.

### Dataset 3: Repair/replay cases

Purpose: evaluate whether repair semantics are not merely descriptive.

Case types:

- Undo broad update.
- Rehydrate derived report from source rows.
- Correct partial backfill.
- Identify non-replayable repair due to missing evidence.
- Detect frame violation where repair touches unrelated rows.

Manifest schema:

```json
{
  "case_id": "repair-broad-update-001",
  "source": "derived-from-public-incident-or-oss-migration",
  "available_at": "during_repair",
  "initial_state": "fixtures/...",
  "bad_transition": "fixtures/...",
  "repair_plan": "fixtures/...",
  "expected": {
    "replay_status": "verified",
    "frame_obligation": "satisfied",
    "scope_obligation": "satisfied",
    "rollback_available": true
  }
}
```

Initial target sizes:

- Smoke: 3 cases.
- Paper-minimum: 25 cases.
- Strong artifact: 100 cases, with explicit unsupported cases.

### Dataset 4: Semantic regression corpus

Purpose: evaluate historical recurrence detection.

Case types:

- Same semantic shape recurs.
- Same high-risk table recurs.
- Same derived report is damaged again.
- False positive near-match that should not be considered recurrence.
- Archive entry with insufficient evidence.

Manifest schema:

```json
{
  "archive_id": "semantic-regression-v1",
  "ordered_incidents": ["incident-a", "incident-b"],
  "expected_regressions": [
    {
      "incident_id": "incident-b",
      "prior_incident_id": "incident-a",
      "relation": "same_semantic_shape",
      "severity": "high"
    }
  ],
  "expected_non_regressions": [
    {
      "incident_id": "incident-c",
      "prior_incident_id": "incident-a",
      "reason": "same table but safe scoped transition"
    }
  ]
}
```

### Implementation tasks

1. Add benchmark manifest types in Go.
2. Add validation command:

   ```bash
   patchline benchmark validate benchmarks/manifests/public_migrations.json
   ```

3. Add runner command:

   ```bash
   patchline benchmark run benchmarks/manifests/public_migrations.json --out results/
   ```

4. Add expected-output comparison:

   ```bash
   patchline artifact-benchmark compare results/... benchmarks/expected/...
   ```

5. Add Make targets:

   ```make
   artifact-smoke
   artifact-benchmark
   artifact-benchmark-refresh
   artifact-benchmark-compare
   ```

### Current status

- Done: manifest structs, validation, runner, comparison, smoke/negative manifests, `public_migrations.json`, committed golden reports, hash-integrity comparison, `make artifact-benchmark-compare`, `make artifact-benchmark-public`, and `make artifact-benchmark-refresh`.
- Next: expand from public migrations to `public_incidents.json` with pinned/cacheable public source-derived observations.

### Acceptance criteria

- The benchmark suite is public-source-derived.
- Every benchmark case has a ground-truth file.
- Every ground-truth file includes source, label, phase, and expected Patchline behavior.
- The benchmark can run without network if cache is present.

## 4. Workstream C: Ground Truth and Labeling Protocol

### Goal

Make all evaluation claims externally checkable and reproducible.

### Labeling rules

Create `benchmarks/LABELING.md` with:

1. Label definitions.
   - `broad_update`
   - `destructive_transition`
   - `missing_rollback`
   - `unsafe_repair`
   - `damaged_derived_output`
   - `semantic_recurrence`
   - `insufficient_evidence`

2. Phase definitions.
   - `pre_deploy`
   - `during_migration`
   - `during_repair`
   - `postmortem`
   - `archive_only`

3. Evidence rules.
   - A label must cite a source URL, file path, commit, or deterministic fixture.
   - If source text is used, store a hash and short permissible excerpt when legally safe.
   - If a rule label is used, cite the rule and the exact statement.
   - If a human interpretation is used, mark it as `human_labeled` and include rationale.

4. Disagreement handling.
   - `ambiguous` cases are excluded from precision/recall calculations by default.
   - `insufficient_evidence` cases are included in negative-case demos.

### Ground-truth file schema

```json
{
  "case_id": "string",
  "case_type": "migration|incident|repair|regression",
  "phase": "pre_deploy|during_migration|during_repair|postmortem|archive_only",
  "labels": {
    "expected_result": "flag|pass|cannot_prove|insufficient_evidence",
    "risk": "info|low|medium|high"
  },
  "evidence": [
    {
      "kind": "url|commit|file|rule|hash|postmortem",
      "locator": "string",
      "sha256": "optional",
      "rationale": "string"
    }
  ],
  "allowed_inputs": [
    "migration_text",
    "schema",
    "policy",
    "trace",
    "repair_plan",
    "archive_prior_to_case"
  ],
  "excluded_inputs": [
    "postmortem_text_if_pre_deploy_claim"
  ]
}
```

### Validation rules

Add a validation command that fails if:

- A case has no source.
- A case has no phase.
- A pre-deploy claim uses postmortem-only evidence.
- An expected result has no rationale.
- A public URL lacks a pinned ref or hash when applicable.
- A case is marked non-ambiguous but has contradictory labels.

### Acceptance criteria

- `make artifact-ground-truth-check` validates every label file.
- README claims link to exact benchmark result files.
- No “would have avoided” claim is made without phase-compatible evidence.

## 5. Workstream D: Baselines

### Goal

Answer the reviewer question: “better than what?”

### Baseline 1: SQL grep/rule baseline

Detect:

- `UPDATE` without `WHERE`
- `DELETE` without `WHERE`
- `DROP TABLE`
- `ALTER TABLE DROP COLUMN`
- unbounded backfill patterns

Command:

```bash
patchline baseline sql-rules benchmarks/manifests/public_migrations.json --out results/baselines/sql-rules.json
```

Expected output:

```json
{
  "baseline": "sql-rules",
  "cases": [
    {
      "case_id": "...",
      "flagged": true,
      "matched_rules": ["update_without_where"]
    }
  ]
}
```

### Baseline 2: Migration-only Patchline

Run Patchline without provenance, replay, archive memory, or solver obligations.

Command:

```bash
patchline benchmark run ... --mode migration-only
```

### Baseline 3: Archive-free Patchline

Run all local analysis but disable semantic regression history.

Command:

```bash
patchline benchmark run ... --mode no-archive
```

### Baseline 4: Solver-free Patchline

Run analysis without Z3-backed obligations. This demonstrates the value of actual solver-backed checks.

Command:

```bash
patchline benchmark run ... --mode no-solver
```

### Metrics

For detection tasks:

- true positives,
- false positives,
- true negatives,
- false negatives,
- precision,
- recall,
- F1.

For actionability:

- whether output contains a touched table,
- whether output contains a frame/scope obligation,
- whether output includes rollback status,
- whether output links to prior incident memory,
- whether output emits replay/proof hash.

For reviewer-facing usefulness:

- number of cases where Patchline provides an actionable next step and baseline only flags a risk.
- number of cases where archive memory upgrades severity or explains recurrence.

### Acceptance criteria

- `make artifact-baselines` emits a Markdown and JSON comparison.
- README includes a small comparison table.
- Full Patchline must outperform at least one baseline on actionability, not only raw detection.

## 6. Workstream E: Ablation Study

### Goal

Show why Patchline’s components belong together.

### Ablation modes

1. `migration-only`
   - SQL/migration risk classification only.

2. `migration+policy`
   - migration risk plus declared policy gate.

3. `migration+policy+solver`
   - adds Z3-backed obligations.

4. `repair-only`
   - repair replay without historical archive.

5. `archive-only`
   - historical recurrence without local repair replay/proof.

6. `full`
   - evidence, transition, policy, solver, repair, replay, archive, semantic regression.

### Output schema

```json
{
  "dataset": "public-oss-migrations-v1",
  "mode": "full",
  "metrics": {
    "precision": 0.0,
    "recall": 0.0,
    "f1": 0.0,
    "actionable_cases": 0,
    "proof_backed_cases": 0,
    "archive_enriched_cases": 0
  },
  "case_results": []
}
```

### Paper table

Generate:

| Mode | Precision | Recall | Actionable cases | Proof-backed | Archive-enriched |
| --- | ---: | ---: | ---: | ---: | ---: |
| migration-only | | | | | |
| migration+policy | | | | | |
| migration+policy+solver | | | | | |
| repair-only | | | | | |
| archive-only | | | | | |
| full | | | | | |

### Acceptance criteria

- `make artifact-ablations` runs all modes on the smoke dataset.
- `make artifact-ablations-full` runs all modes on the full benchmark.
- At least one documented case shows a baseline finding upgraded into a proof-backed or archive-backed actionable finding.

## 7. Workstream F: Time-Realistic Counterfactuals

### Goal

Prevent hindsight leakage in historical claims.

### Phase model

Add a `phase` package or data model:

```go
type AvailabilityPhase string

const (
    PreDeploy AvailabilityPhase = "pre_deploy"
    DuringMigration AvailabilityPhase = "during_migration"
    DuringRepair AvailabilityPhase = "during_repair"
    Postmortem AvailabilityPhase = "postmortem"
    ArchiveOnly AvailabilityPhase = "archive_only"
)
```

Every artifact input should declare `available_at`.

### Phase rules

1. Pre-deploy evaluation may use:
   - migration text,
   - schema,
   - declared policy,
   - prior archive entries,
   - known invariants.

2. Pre-deploy evaluation may not use:
   - later incident narrative,
   - actual damaged row counts unless statically available,
   - postmortem conclusions.

3. During-repair evaluation may use:
   - observed bad state,
   - repair plan,
   - available backups,
   - trace/replay logs.

4. Postmortem/archive evaluation may use:
   - public incident narrative,
   - final root cause,
   - postmortem repair outcomes.

### Implementation tasks

1. Add `available_at` to archive entries and benchmark cases.
2. Add phase-checking validation.
3. Add command:

   ```bash
   patchline phase-check benchmarks/manifests/public_incidents.json
   ```

4. Add tests for preventing pre-deploy/postmortem leakage.
5. Update historical counterfactual claims to state phase explicitly.

### Acceptance criteria

- Historical demos print their phase.
- Pre-deploy claims fail validation if they cite postmortem-only evidence.
- README uses precise phrases:
  - “Patchline would have flagged the transition pre-deploy.”
  - “Patchline would have made the repair replay obligation explicit during repair.”
  - “Patchline can use the postmortem as archive memory afterward.”

## 8. Workstream G: Artifact Packaging

### Goal

Make the artifact easy for reviewers to run.

### Required files

1. `ARTIFACT.md`
2. `Dockerfile`
3. `.devcontainer/devcontainer.json`
4. `scripts/artifact_smoke.sh`
5. `scripts/artifact_full.sh`
6. `benchmarks/cache/README.md`
7. `results/README.md`

### Make targets

```make
artifact-smoke:
	go test ./...
	go run ./cmd/patchline semantic-audit --json
	go run ./cmd/patchline semantic-regressions examples/archive/bad-migration-corpus.json --json
	go run ./cmd/patchline benchmark run benchmarks/manifests/smoke.json --out results/smoke

artifact-full:
	$(MAKE) artifact-ground-truth-check
	$(MAKE) artifact-benchmark
	$(MAKE) artifact-baselines
	$(MAKE) artifact-ablations
	$(MAKE) artifact-scale

artifact-clean:
	rm -rf results/generated
```

### Docker image requirements

Install:

- Go version used by the repo.
- Z3 CLI/library as required.
- jq.
- git.
- curl.
- make.
- ca-certificates.

The Docker image should support:

```bash
docker build -t patchline-artifact .
docker run --rm patchline-artifact make artifact-smoke
```

### Acceptance criteria

- Fresh clone path works.
- Docker path works.
- No live network is needed for smoke.
- Full path documents when network is used and where data is cached.
- Expected runtime is printed at the beginning of each artifact target.

## 9. Workstream H: Scale and Performance

### Goal

Show Patchline can handle more than toy examples.

### Measurements

Add benchmark instrumentation for:

- number of migration files,
- number of SQL statements,
- number of archive entries,
- number of derived reports,
- number of repair cases,
- trace event count,
- wall-clock runtime,
- max RSS if available,
- Z3 obligation count,
- Z3 solve time,
- archive query latency,
- semantic regression count.

### Command

```bash
patchline artifact measure benchmarks/manifests/public_migrations.json --out results/scale/public_migrations.json
```

### Output schema

```json
{
  "dataset": "public-oss-migrations-v1",
  "started_at": "...",
  "finished_at": "...",
  "counts": {
    "files": 0,
    "statements": 0,
    "archive_entries": 0,
    "solver_obligations": 0
  },
  "timings_ms": {
    "ingest": 0,
    "migration_analysis": 0,
    "solver": 0,
    "archive_queries": 0,
    "semantic_regressions": 0,
    "total": 0
  }
}
```

### README table

| Dataset | Files | Statements | Incidents | Obligations | Total time | Query time |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| smoke | | | | | | |
| public migrations | | | | | | |
| public incidents | | | | | | |

### Acceptance criteria

- `make artifact-scale` emits JSON and Markdown.
- Scale results are deterministic enough for artifact review.
- README reports exact commit and machine environment for generated numbers.

## 10. Workstream I: Trusted Semantic Core

### Goal

Make the semantics auditable and not scattered across demos.

### Core model document

Create `docs/semantic-core.md` with:

1. State model.
   - source state,
   - derived state,
   - damaged state,
   - repaired state,
   - archive state.

2. Transition model.
   - migration transition,
   - repair transition,
   - rollback transition,
   - replay transition,
   - archive update transition.

3. Obligations.
   - scope,
   - frame,
   - rollback,
   - replay determinism,
   - invariant preservation,
   - recurrence review.

4. Z3 mapping.
   - Which obligations are encoded in Z3.
   - Which are not solver-backed and why.
   - How solver results are represented.

5. Implementation map.

| Semantic concept | Go package/file | CLI command | Tests |
| --- | --- | --- | --- |
| Trace | `internal/replay`, `internal/provenance` | `trace-reconstruct` | |
| Migration transition | `internal/migration` | `analyze-migration` | |
| Repair transition | `internal/repair` | `repair-semantics` | |
| Proof obligation | `internal/solver` | `solver-obligations` | |
| Archive memory | `internal/archive` | `archive-query` | |
| Regression | `internal/archive` | `semantic-regressions` | |

### Code tasks

1. Introduce explicit semantic artifact structs if not already centralized.
2. Ensure every CLI output includes:
   - `artifact_kind`,
   - `artifact_version`,
   - `artifact_hash`,
   - `inputs_hash`.
3. Add tests for hash stability.
4. Add a semantic contract test that fails if commands drift from the documented core.

### Acceptance criteria

- A reviewer can inspect `docs/semantic-core.md` and know where each concept lives in code.
- The semantic audit includes the newly documented core artifacts.
- CLI JSON outputs are stable enough to use as paper evidence.

## 11. Workstream J: Negative Cases and Limitations

### Goal

Increase credibility by showing where Patchline refuses to overclaim.

### Negative case types

1. Unsupported SQL.
   - Example: dialect-specific procedural migration that parser cannot model.
   - Expected: `unsupported_fragment`.

2. Insufficient evidence.
   - Example: postmortem says data corruption occurred but gives no transition details.
   - Expected: `insufficient_evidence`.

3. Phase leakage.
   - Example: pre-deploy claim tries to use postmortem root cause.
   - Expected: validation failure.

4. Non-replayable repair.
   - Example: repair depends on missing backup.
   - Expected: `cannot_replay`.

5. False recurrence.
   - Example: same table touched safely with scoped predicate and proof.
   - Expected: no semantic regression.

### Implementation tasks

1. Add `examples/negative/`.
2. Add `patchline negative-cases` command or benchmark mode.
3. Add `make artifact-negative-cases`.
4. Document limitations in README and `ARTIFACT.md`.

### Acceptance criteria

- Negative cases are part of validation, not buried in prose.
- At least five negative cases are executable.
- The artifact explicitly distinguishes:
  - detected risk,
  - proven violation,
  - cannot prove,
  - insufficient evidence.

## 12. Workstream K: Literature Positioning and Novelty

### Goal

Make the novelty legible to software engineering reviewers.

### Document

Create `docs/literature-positioning.md`.

### Positioning claims

Patchline is not:

- a generic migration linter,
- a provenance-only debugger,
- a program repair tool,
- a postmortem management tool,
- a database verification prototype only,
- an AI incident assistant.

Patchline is:

- a semantics-first repair artifact system,
- a bridge between historical incident evidence and executable future gates,
- a way to make production data repair behavior replayable, checkable, and reusable.

### Comparison table

| Area | What existing systems do | What they usually do not do | Patchline contribution |
| --- | --- | --- | --- |
| Data provenance | Explain where data came from | Encode repair obligations and recurrence memory | Connect provenance to repair replay and archive gates |
| Migration linting | Flag risky SQL patterns | Join risk to repair replay/proof/history | Semantic artifact pipeline |
| Program repair | Generate/fix code | Model production data repair and rollback | Repair semantics for data state |
| Incident management | Store narratives | Make narratives executable | Source-grounded semantic archive |
| DB verification | Prove query/transaction properties | Package operational incidents as reusable artifacts | Artifact-level repair obligations |

### Acceptance criteria

- README novelty section is concise and points to the deeper doc.
- Literature positioning does not overclaim ignorance of related work.
- The artifact contribution is stated as an integration with a new executable object, not as a hodge-podge.

## 13. Workstream L: Canonical Killer Demo

### Goal

Provide one command reviewers can run and cite.

### Command

```bash
make artifact-demo
```

### Desired output

```text
Patchline artifact demo
dataset: public-incidents-smoke-v1
claim: historical repair semantics detects recurrence and proves repair obligations

[1/7] verifying source evidence ... ok
[2/7] reconstructing trace ... ok hash=...
[3/7] analyzing bad transition ... gate=fail risk=high
[4/7] generating Z3 obligations ... fail/sat/unsat summary=...
[5/7] replaying repair ... verification=verified hash=...
[6/7] updating archive ... entries=...
[7/7] detecting semantic regression ... count=...

artifact_bundle_hash=...
paper_table=results/artifact-demo/summary.md
```

### Demo requirements

- Uses public or committed smoke data.
- Emits JSON and Markdown.
- Has stable expected outputs.
- Demonstrates:
  - source grounding,
  - transition risk,
  - Z3-backed obligation,
  - repair replay,
  - archive recurrence.

### Acceptance criteria

- `make artifact-demo` completes under 5 minutes.
- The README includes the exact output shape.
- The generated bundle hash is deterministic.

## 14. Concrete Implementation Order

### Phase 1: Artifact framing and reviewer path

1. Add `ARTIFACT.md`.
2. Add `docs/paper-claim.md`.
3. Add `docs/semantic-core.md`.
4. Add `docs/semantic-pipeline.md`.
5. Revise README around the central claim and pipeline.
6. Add `make artifact-smoke`.

Reason: this immediately makes the repo easier to understand and review.

### Phase 2: Benchmark skeleton

1. Add `benchmarks/README.md`.
2. Add `benchmarks/LABELING.md`.
3. Add manifest schemas.
4. Add smoke manifests.
5. Add ground-truth validation command.
6. Add `make artifact-ground-truth-check`.

Reason: this prevents future examples from looking synthetic or ungrounded.

### Phase 3: Public migration benchmark

1. Select 5 public repos.
2. Pin commits.
3. Extract migration files.
4. Label risky statements.
5. Run Patchline and SQL-rule baseline.
6. Generate comparison table.

Reason: migration corpora are likely easiest to scale quickly.

### Phase 4: Public incident benchmark

1. Select 10 public incidents.
2. Extract phase-aware labels.
3. Encode counterfactual claims.
4. Validate evidence availability.
5. Run Patchline historical gates.
6. Generate claim table.

Reason: this directly addresses the artifact-paper motivation.

### Phase 5: Baselines and ablations

1. Implement SQL-rule baseline.
2. Implement benchmark modes.
3. Generate metrics.
4. Add output comparison.
5. Add README summary table.

Reason: this answers the strongest reviewer objection.

### Phase 6: Packaging

1. Add Dockerfile.
2. Add devcontainer.
3. Add smoke/full scripts.
4. Add cached-data instructions.
5. Test fresh checkout path.

Reason: artifact review requires reproducibility more than feature breadth.

### Phase 7: Scale and negative cases

1. Add scale measurement.
2. Add negative cases.
3. Add limitations section.
4. Add performance table.

Reason: this makes the artifact credible and honest.

### Phase 8: Paper-facing polish

1. Add generated tables under `results/`.
2. Add a paper-introduction motivation draft.
3. Add citation/literature positioning.
4. Ensure README, `ARTIFACT.md`, and docs make identical claims.

Reason: this connects engineering work to conference expectations.

## 15. Proposed `make` Target Set

```make
artifact-smoke
artifact-demo
artifact-ground-truth-check
artifact-benchmark
artifact-baselines
artifact-ablations
artifact-scale
artifact-negative-cases
artifact-full
artifact-clean
```

### Expected target meanings

| Target | Runtime target | Network? | Purpose |
| --- | ---: | --- | --- |
| `artifact-smoke` | < 5 min | No | Artifact evaluator quick check |
| `artifact-demo` | < 5 min | No | Canonical paper demo |
| `artifact-ground-truth-check` | < 1 min | No | Validate labels and phases |
| `artifact-benchmark` | 5-30 min | Optional | Run public benchmark |
| `artifact-baselines` | 5-15 min | Optional | Compare baseline tools |
| `artifact-ablations` | 5-20 min | Optional | Show component contribution |
| `artifact-scale` | 5-30 min | Optional | Measure throughput/latency |
| `artifact-negative-cases` | < 5 min | No | Show honest limitations |
| `artifact-full` | 30-90 min | Optional | Reproduce paper tables |

## 16. Reviewer-Facing Result Tables to Generate

### Table 1: Benchmark corpus

| Corpus | Cases | Public source | Ground truth type | Phase-aware? |
| --- | ---: | --- | --- | --- |
| OSS migrations | | GitHub pinned commits | rule/human labels | yes |
| Public incidents | | public postmortems | source-grounded labels | yes |
| Repair cases | | public-derived fixtures | expected replay/proof status | yes |
| Semantic regressions | | archive cases | expected recurrence labels | yes |

### Table 2: Detection performance

| Method | Precision | Recall | F1 | Actionable cases |
| --- | ---: | ---: | ---: | ---: |
| SQL rules | | | | |
| Migration-only Patchline | | | | |
| Full Patchline | | | | |

### Table 3: Ablation

| Mode | Cases flagged | Proof-backed | Replay-backed | Archive-enriched |
| --- | ---: | ---: | ---: | ---: |
| migration-only | | | | |
| migration+policy | | | | |
| migration+policy+solver | | | | |
| repair-only | | | | |
| archive-only | | | | |
| full | | | | |

### Table 4: Historical counterfactuals

| Incident | Claim phase | Evidence source | Patchline result | Hindsight-safe? |
| --- | --- | --- | --- | --- |
| GitLab 2017 | pre-deploy/during-repair/postmortem | public postmortem | | |

### Table 5: Scale

| Dataset | Files | Statements | Incidents | Obligations | Runtime | Memory |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| smoke | | | | | | |
| full | | | | | | |

## 17. Definition of Done for Artifact-Paper Readiness

Patchline is artifact-paper-ready when:

1. The README and `ARTIFACT.md` state one central claim.
2. `make artifact-smoke` works from a fresh clone.
3. `make artifact-demo` produces a deterministic bundle hash.
4. Public benchmark manifests exist and validate.
5. Ground-truth labels are source-grounded and phase-aware.
6. Baselines and ablations run from Make targets.
7. At least one public migration corpus and one public incident corpus are included.
8. The full pipeline demonstrably adds actionability beyond baseline risk flags.
9. Z3-backed obligations are explicitly used and reported.
10. Negative cases show honest boundaries.
11. Scale numbers are generated and documented.
12. The semantic core is documented and mapped to code.
13. Literature positioning explains novelty without claiming unrelated tools do not exist.
14. Paper-facing result tables can be regenerated with one command.

## 18. Immediate Next Commit Plan

The artifact-review scaffold, executable benchmark runner, golden-refresh workflow, and pinned public migration corpus are now in place. The next implementation commit should make the benchmark suite cover public incidents with the same phase-safe manifest protocol:

1. Add `benchmarks/manifests/public_incidents.json`.
2. Add source-derived observation fixtures from at least two public incidents with pinned evidence URLs/hashes.
3. Include at least one positive recurrence/flag case and one insufficient-evidence boundary case.
4. Keep fresh-checkout smoke validation offline; put network-backed fetching behind an explicit public-incident target.
5. Add committed expected reports only after the manifest can run deterministically from cached fixtures.

Commit message:

```text
Add pinned public incident benchmark manifest

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

## 19. Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| Public data is too messy | Start with small pinned smoke data, then scale |
| Historical incidents lack enough detail | Label as postmortem/archive only or insufficient evidence |
| Benchmarks look handpicked | Use selection protocol and include negative cases |
| Baselines are weak strawmen | Include simple but credible rule baselines and Patchline ablations |
| Z3 use appears superficial | Map each solver-backed obligation to a specific artifact claim |
| Claims use hindsight | Enforce `available_at` and phase checks |
| Artifact is too hard to run | Docker, smoke target, cached fixtures |
| Repo feels too broad | Keep README centered on the semantic artifact pipeline |

## 20. Final Artifact Paper Shape

The eventual paper should read like:

1. **Motivation:** production data repair incidents recur because postmortems are prose, migration gates are local, and repair correctness is rarely replayable.
2. **Idea:** convert repair incidents into executable semantic artifacts.
3. **System:** Patchline implements evidence, trace, transition, repair, proof, replay, archive, and regression stages.
4. **Artifact:** public benchmarks, Dockerized commands, deterministic hashes.
5. **Evaluation:** public migration corpus, public incident corpus, repair/replay cases, semantic recurrence cases.
6. **Results:** Patchline improves actionability over baselines and detects recurrence using historical semantic memory.
7. **Limitations:** phase availability, unsupported SQL, incomplete evidence, bounded replay.
8. **Reproducibility:** `make artifact-smoke`, `make artifact-demo`, `make artifact-full`.

The codebase should be engineered so every one of those sections has a corresponding runnable command and committed expected output.
