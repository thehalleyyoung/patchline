# Limitations ledger

`patchline repo limitations-ledger` reads one or more `repo analyze` outputs and writes an explicit ledger of limitations that should remain visible in research claims, maintainer review, and generated-code evaluation.

```bash
go run ./cmd/patchline repo limitations-ledger \
  --analyses results/generated/limitations-ledger-gate/analyses/lobsters-rails-migrations,results/generated/limitations-ledger-gate/analyses/forem-rails-migrations \
  --out results/generated/limitations-ledger-gate/ledger \
  --json
```

The JSON report writes `limitations-ledger.json` with:

- `limitations[]`: individual limitations with `category`, `severity`, `observation`, `evidence`, `why_it_matters`, `not_a_claim`, `next_evidence`, and `affected_artifacts`.
- `categories[]`: definitions and review rules for `unsupported_ecosystem`, `uncertain_causality`, `missing_runtime_evidence`, and `intentionally_conservative_check`.
- `summary`: counts for analyses, public repos, total limitations, `unsupported_ecosystems`, `uncertain_causality`, `missing_runtime_evidence`, `intentionally_conservative_checks`, and per-category totals.
- `corpus`: the public repository slices used as evidence.

The ledger is deliberately conservative. `unsupported_ecosystem` means the analyzed slice has partial framework or native-command evidence, not that the whole repository is unusable. `uncertain_causality` means links are useful navigation, not causal proof. `missing_runtime_evidence` keeps static proof holes separate from runtime guarantees. `intentionally_conservative_check` records places where Patchline blocks, warns, or preserves proof holes because weakening the check would overstate the evidence.

`make limitations-ledger-gate` validates the command on pinned public repository slices and requires all four categories with explicit not-a-claim boundaries.
