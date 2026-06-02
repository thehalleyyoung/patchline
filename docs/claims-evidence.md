# Claims-to-evidence map

`patchline repo claims-evidence` reads one or more `repo analyze` outputs and writes a paper-facing map from future abstract, introduction, and evaluation claims to concrete evidence artifacts.

```bash
go run ./cmd/patchline repo claims-evidence \
  --analyses results/generated/claims-evidence-gate/analyses/lobsters-rails-migrations,results/generated/claims-evidence-gate/analyses/grafana-go-migrations \
  --out results/generated/claims-evidence-gate/claims \
  --json
```

The JSON report writes `claims-evidence.json` with:

- `claims[]`: each claim has `section`, `claim`, `status`, `evidence`, `artifacts`, `limitations`, `missing_evidence`, `paper_wording`, `reviewer_check`, `affected_repos`, `required_for_paper`, and `expected_paper_slot`.
- `sections[]`: claim policies for `abstract`, `introduction`, and `evaluation`.
- `summary`: counts for analyses, public repos, claims, per-section claims, supported/qualified/unsupported claims, and claims with limitations.
- `corpus`: pinned repositories, refs, and subpaths used as evidence.

The command is intentionally conservative: it maps what the artifacts support, then lists limitations and missing evidence so a future paper does not overclaim causality, runtime safety, ecosystem coverage, or repair completion.

`make claims-evidence-gate` validates the report on pinned public repository slices and requires claim coverage for all three paper sections with artifact paths and limitations.
