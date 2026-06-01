# Patchline benchmark scaffold

This directory contains the artifact-paper benchmark scaffold. It is intentionally small in the first artifact path, but it uses the same schema expected for larger public corpora.

## Layout

```text
benchmarks/
  manifests/       dataset manifests that point to fixtures and ground truth
  ground_truth/    source-grounded labels with phase metadata
  expected/        frozen expected reports for executable manifest runs
  cache/           optional local cache for downloaded public corpora
  corpus_protocol.json executable selection/candidate-ledger audit protocol
```

## Smoke datasets

`manifests/smoke.json` covers four committed cases:

1. a pre-deploy bad migration classification;
2. a postmortem public incident label;
3. a during-repair replay/proof label;
4. an archive-only semantic regression label.

The smoke set is intentionally small. Its purpose is to make the artifact protocol executable before adding larger public corpora.

## Executable manifests and golden reports

Run the committed benchmark manifests and compare them to expected outputs with:

```bash
make artifact-benchmark-compare
```

The target writes generated reports to `results/generated/artifact-benchmark/` and compares them with:

```text
benchmarks/expected/smoke-report.json
benchmarks/expected/negative-report.json
benchmarks/expected/repair-cases-report.json
benchmarks/expected/semantic-regressions-report.json
```

The runner is phase-aware. It validates that each manifest case points to matching ground truth, checks `available_at` against the ground-truth phase, enforces `allowed_inputs` and `excluded_inputs`, rejects allowed inputs whose earliest availability is after the case phase, predicts from committed fixtures or explicit inline fixtures, and only then compares the result with the expected label. This keeps pre-deploy cases from using postmortem-only facts. Repair cases may also declare explicit bounded stores and invariant specs, which makes replay hashes, solver hashes, and invariant counts part of the benchmark result instead of hidden defaults.

For a direct reviewer check of the no-hindsight contract without running case predictions, use:

```bash
make phase-check
```

This runs `patchline phase-check <manifest.json>` over every committed manifest and reports each case's declared phase and fixture input kind before applying the same ground-truth phase/input availability validation used by `artifact-benchmark validate`.

The offline compare target now covers:

- `manifests/smoke.json` for a compact end-to-end artifact path;
- `manifests/negative.json` for unsupported/insufficient/phase-boundary controls;
- `manifests/repair_cases.json` for during-repair replay and proof checks;
- `manifests/semantic_regressions.json` for archive-only recurrence and non-recurrence checks.

Run the pinned public migration corpus with:

```bash
make artifact-benchmark-public
```

That target fetches five Bytebase migrations at a pinned commit from `examples/public-corpus/sources.json`, verifies their source hashes, writes `results/generated/public-corpus/fetch-report.json`, runs the legacy label/hash benchmark suite, then runs `manifests/public_migrations.json` against `expected/public-migrations-report.json`. Set `PATCHLINE_PUBLIC_CORPUS_OFFLINE=1` to make the fetch step validate the local cache and fail instead of downloading a missing or corrupted file. The public manifest is marked `requires_fetch`, so ground-truth validation can remain offline while the public benchmark target deliberately exercises network-backed real-world data.

Run the offline public-incident corpus with:

```bash
make artifact-benchmark-public-incidents
```

That target runs `manifests/public_incidents.json` against GitLab 2017 and GitHub 2018 source-observation JSONL fixtures, then checks an adjacent boundary case where a public summary remains too thin and must produce `insufficient_evidence`. Unlike `artifact-benchmark-public`, this target does not fetch network data.

Run the public-derived repair boundary with:

```bash
make artifact-benchmark-public-repairs
```

That target runs `manifests/public_repairs.json` against one Patchline-authored counterfactual repair manifest derived from GitLab 2017 public sources, then compares against `expected/public-repairs-report.json`. The public postmortem and follow-up issues are the source evidence; the local repair manifest is a derived input artifact. This is intentionally not framed as a full repair corpus yet.

Run the paired public-derived archive boundary with:

```bash
make artifact-benchmark-public-archive
```

That target runs `manifests/public_archive.json` against `examples/archive/public-postmortem-derived-paired-archive.json`, then compares against `expected/public-archive-report.json`. The case uses public GitLab/GitHub postmortem-derived observations, but the SQL, repair manifests, and replay stores are explicitly local semantic reconstructions; this keeps the benchmark executable without claiming access to private production rows.

`make artifact-demo` includes this same public-derived archive path in the canonical reviewer bundle. It writes the phase check, archive index/query, semantic-regression report, benchmark validation/run/compare JSON, and a `bundle-manifest.json` with per-file SHA-256 hashes under `results/generated/artifact-demo/`, so the demo is no longer only a private smoke fixture.

Expected reports are intentionally not refreshed by the compare target. After a deliberate semantics change, maintainers can run:

```bash
PATCHLINE_ACCEPT_EXPECTED_REFRESH=1 make artifact-benchmark-refresh
```

That maintainer-only target rewrites the committed golden JSON/Markdown reports, including repair, regression, public migration, public incident, public-derived repair, and public-derived archive reports. It refuses to run unless `PATCHLINE_ACCEPT_EXPECTED_REFRESH=1` is set, and deliberately does not present a compare against just-refreshed files as validation; run the normal compare paths separately after reviewing the expected-output diff.

## Executable study outputs

The committed strict corpus under `examples/benchmarks/strict-migration-corpus.json` now feeds:

```bash
make artifact-studies
make artifact-studies-compare
```

Those targets emit baseline, ablation, and scale reports under `results/generated/artifact-studies/`, then compare their stable hashes against `benchmarks/expected/studies-strict.json`. The reports are deliberately small but load-bearing: they verify that benchmark cases can carry ground-truth links, archive links, repair manifests, invariant specs, Z3-backed obligations, and deterministic migration hashes through the same schema that larger corpora will use.

The independent baseline set is intentionally transparent and deterministic: raw-SQL DDL-grep, raw-SQL normalized rules, and an externally grounded migration-guardrail linter rule pack. The guardrail linter is not an oracle; it applies fixed rules for broad rewrites, destructive drops, and persistent data inserts. The Patchline effects-only row is labeled as an ablation because it intentionally consumes the Patchline analyzer while omitting evidence/proof/archive links.

The pinned public migration corpus feeds the same study machinery through an explicit network target:

```bash
make artifact-studies-public
make artifact-studies-public-compare
```

Those targets write reports under `results/generated/artifact-studies/public-migrations/` after fetching and hashing the Bytebase files, then compare their stable hashes against `benchmarks/expected/studies-public-migrations.json`. Because this corpus only declares migration files, its reports are scoped to detection and structural actionability; archive/proof/ground-truth gains remain demonstrated by the strict corpus and the phase-aware benchmark manifests.

The study comparison path deliberately compares each report's canonical `hash` instead of exact JSON bytes. This keeps the scale study drift-detecting while ignoring local timing noise. To refresh expected study hashes after an intentional semantic change, run:

```bash
PATCHLINE_ACCEPT_EXPECTED_REFRESH=1 make artifact-studies-refresh
```

That maintainer-only refresh target rewrites the expected study manifests only, and refuses to run unless `PATCHLINE_ACCEPT_EXPECTED_REFRESH=1` is set. Run `make artifact-studies-compare` and `make artifact-studies-public-compare` separately after reviewing the manifest diff.

## Paper-facing table generation

Regenerate the reviewer-facing result tables with:

```bash
make artifact-tables
```

This writes `results/generated/artifact-tables/summary.json` and `summary.md`. The generator derives five paper tables from checked study specs and regenerated benchmark reports after their hashes match the frozen expected reports: executable corpora, detection/actionability, ablation, historical public-derived counterfactuals, and scale. It records actual/expected source hashes and keeps public incident/archive claims scoped to public-postmortem-derived semantic reconstructions.

Regenerate the complete reportable-number ledger with:

```bash
make artifact-numbers
```

This writes `results/generated/artifact-numbers/summary.json` and `summary.md`, including every per-case baseline, ablation, scale, regenerated benchmark outcome, matching expected-report hash, and input hash behind the aggregate tables.

Regenerate the explicit defined-subtask win report with:

```bash
make artifact-subtasks
```

This writes `results/generated/artifact-subtasks/summary.json` and `summary.md`. The report consumes the checked experiment-number ledger and fails if the public Bytebase migration-risk or strict repair-review comparison no longer shows Patchline beating the declared raw-SQL, guardrail, and effects-only comparators on the stated metrics.

Audit benchmark selection and candidate accounting with:

```bash
make artifact-corpus-audit
```

This writes `results/generated/artifact-corpus-audit/summary.json` and `summary.md`. The report hashes `corpus_protocol.json`, verifies every committed manifest is covered, checks included cases against candidate pools, reports result and boundary coverage, validates required evidence kinds, and ensures reviewer commands validate rather than refresh expected outputs.

Regenerate the artifact-wide provenance receipt with:

```bash
make artifact-provenance
```

This writes `results/generated/artifact-provenance/summary.json` and `summary.md`. The report hashes benchmark manifests, ground truth, frozen expected outputs, generated reviewer reports, the public corpus source manifest, and the demo bundle manifest; it also independently re-hashes manifest-derived cached public corpus files and rejects demo bundles with missing or extra stage outputs.

## Larger benchmark targets

The planned full benchmark families are:

- public OSS migrations (currently five pinned Bytebase migrations);
- public historical incidents (currently GitLab 2017 and GitHub 2018 source-observation cases plus an insufficient-evidence boundary);
- public-derived repair boundaries (currently one GitLab 2017 counterfactual no-snapshot repair case);
- paired public-derived archives (currently one GitLab/GitHub postmortem-derived recurrence case);
- repair/replay cases (currently three committed during-repair cases);
- semantic regression archives (currently one recurrence and one non-recurrence case);
- negative/limitation cases.

Each case must have a ground-truth JSON file with source, phase, allowed inputs, excluded inputs, expected result, and evidence rationale. `benchmarks/LABELING.md` defines the accepted phase names and the earliest phase for every allowed input kind.
