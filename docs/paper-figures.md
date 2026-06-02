# Paper figure generation

`patchline repo figures` turns one or more `repo analyze` outputs into reproducible paper figures and machine-readable data files. It writes:

- `repair-analysis-loop.svg` and `repair-analysis-loop.json`
- `architecture.svg` and `architecture.json`
- `corpus-composition.svg` and `corpus-composition.json`
- `ablations.svg` and `ablations.json`
- `intervention-outcomes.svg` and `intervention-outcomes.json`
- `figures.json` and `figures.md`

```bash
bash scripts/generate-paper-figures.sh \
  results/generated/paper-figures-gate/analyses/lobsters-rails-migrations,results/generated/paper-figures-gate/analyses/grafana-go-migrations \
  results/generated/paper-figures-gate/figures
```

The figures are intentionally simple SVGs with adjacent JSON data so reviewers can inspect the exact values behind every bar or loop node. Figure captions and `source_artifacts` link back to `inventory/inventory.json`, `baseline/baseline.json`, `proposal/proposal.json`, and `compare/compare.json`.

The five required figure kinds are `repair_analysis_loop`, `architecture`, `corpus_composition`, `ablations`, and `intervention_outcomes`. They are meant for a future paper's system overview, evaluation setup, ablation discussion, and before/after intervention outcome sections.

`make paper-figures-gate` validates the generator on pinned public repository slices and checks that all SVGs, JSON data files, captions, source artifacts, and required figure kinds are present.
