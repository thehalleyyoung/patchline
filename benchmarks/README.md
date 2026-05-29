# Patchline benchmark scaffold

This directory contains the artifact-paper benchmark scaffold. It is intentionally small in the first artifact path, but it uses the same schema expected for larger public corpora.

## Layout

```text
benchmarks/
  manifests/       dataset manifests that point to fixtures and ground truth
  ground_truth/    source-grounded labels with phase metadata
  expected/        frozen expected reports for executable manifest runs
  cache/           optional local cache for downloaded public corpora
```

## Smoke datasets

`manifests/smoke.json` covers four committed cases:

1. a pre-deploy bad migration classification;
2. a postmortem public incident label;
3. a during-repair replay/proof label;
4. an archive-only semantic regression label.

The smoke set is not a paper-scale evaluation. Its purpose is to make the artifact protocol executable before adding the larger public corpora described in `NEWEST_PLAN.md`.

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

The runner is phase-aware. It validates that each manifest case points to matching ground truth, enforces `allowed_inputs` and `excluded_inputs`, predicts from committed fixtures or explicit inline fixtures, and only then compares the result with the expected label. This keeps pre-deploy cases from using postmortem-only facts. Repair cases may also declare explicit bounded stores and invariant specs, which makes replay hashes, solver hashes, and invariant counts part of the benchmark result instead of hidden defaults.

The offline compare target now covers:

- `manifests/smoke.json` for a compact end-to-end artifact path;
- `manifests/negative.json` for unsupported/insufficient/phase-boundary controls;
- `manifests/repair_cases.json` for during-repair replay and proof checks;
- `manifests/semantic_regressions.json` for archive-only recurrence and non-recurrence checks.

Run the pinned public migration corpus with:

```bash
make artifact-benchmark-public
```

That target fetches five Bytebase migrations at a pinned commit, verifies their source hashes, runs the legacy label/hash benchmark suite, then runs `manifests/public_migrations.json` against `expected/public-migrations-report.json`. The public manifest is marked `requires_fetch`, so ground-truth validation can remain offline while the public benchmark target deliberately exercises network-backed real-world data.

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

Expected reports are intentionally not refreshed by the compare target. After a deliberate semantics change, maintainers can run:

```bash
make artifact-benchmark-refresh
```

That target rewrites the committed golden JSON/Markdown reports, including repair, regression, public migration, public incident, public-derived repair, and public-derived archive reports, and then runs the normal compare paths against the refreshed files.

## Executable study outputs

The committed strict corpus under `examples/benchmarks/strict-migration-corpus.json` now feeds:

```bash
make artifact-studies
make artifact-studies-compare
```

Those targets emit baseline, ablation, and scale reports under `results/generated/artifact-studies/`, then compare their stable hashes against `benchmarks/expected/studies-strict.json`. The reports are deliberately small but load-bearing: they verify that benchmark cases can carry ground-truth links, archive links, repair manifests, invariant specs, Z3-backed obligations, and deterministic migration hashes through the same schema that larger corpora will use.

The pinned public migration corpus feeds the same study machinery through an explicit network target:

```bash
make artifact-studies-public
make artifact-studies-public-compare
```

Those targets write reports under `results/generated/artifact-studies/public-migrations/` after fetching and hashing the Bytebase files, then compare their stable hashes against `benchmarks/expected/studies-public-migrations.json`. Because this corpus only declares migration files, its reports are scoped to detection and structural actionability; archive/proof/ground-truth gains remain demonstrated by the strict corpus and the phase-aware benchmark manifests.

The study comparison path deliberately compares each report's canonical `hash` instead of exact JSON bytes. This keeps the scale study drift-detecting while ignoring local timing noise. To refresh expected study hashes after an intentional semantic change, run:

```bash
make artifact-studies-refresh
```

## Larger benchmark targets

The planned full benchmark families are:

- public OSS migrations (currently five pinned Bytebase migrations);
- public historical incidents (currently GitLab 2017 and GitHub 2018 source-observation cases plus an insufficient-evidence boundary);
- public-derived repair boundaries (currently one GitLab 2017 counterfactual no-snapshot repair case);
- paired public-derived archives (currently one GitLab/GitHub postmortem-derived recurrence case);
- repair/replay cases (currently three committed during-repair cases);
- semantic regression archives (currently one recurrence and one non-recurrence case);
- negative/limitation cases.

Each case must have a ground-truth JSON file with source, phase, allowed inputs, excluded inputs, expected result, and evidence rationale.
