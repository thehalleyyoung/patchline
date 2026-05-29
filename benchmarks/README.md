# Patchline benchmark scaffold

This directory contains the artifact-paper benchmark scaffold. It is intentionally small in the first artifact path, but it uses the same schema expected for larger public corpora.

## Layout

```text
benchmarks/
  manifests/       dataset manifests that point to fixtures and ground truth
  ground_truth/    source-grounded labels with phase metadata
  expected/        expected outputs for future full benchmark runs
  cache/           optional local cache for downloaded public corpora
```

## Smoke datasets

`manifests/smoke.json` covers four committed cases:

1. a pre-deploy bad migration classification;
2. a postmortem public incident label;
3. a during-repair replay/proof label;
4. an archive-only semantic regression label.

The smoke set is not a paper-scale evaluation. Its purpose is to make the artifact protocol executable before adding the larger public corpora described in `NEWEST_PLAN.md`.

## Executable study outputs

The committed strict corpus under `examples/benchmarks/strict-migration-corpus.json` now feeds:

```bash
make artifact-studies
```

That target emits baseline, ablation, and scale reports under `results/generated/artifact-studies/`. The reports are deliberately small but load-bearing: they verify that benchmark cases can carry ground-truth links, archive links, repair manifests, invariant specs, Z3-backed obligations, and deterministic migration hashes through the same schema that larger corpora will use.

## Larger benchmark targets

The planned full benchmark families are:

- public OSS migrations;
- public historical incidents;
- repair/replay cases;
- semantic regression archives;
- negative/limitation cases.

Each case must have a ground-truth JSON file with source, phase, allowed inputs, excluded inputs, expected result, and evidence rationale.
