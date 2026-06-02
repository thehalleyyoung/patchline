# Generated reviewability examples

`patchline repo reviewability-examples` reads `repo analyze` outputs and extracts generated test/guard artifacts that make review easier without claiming to fully repair the underlying data-change risk.

```bash
go run ./cmd/patchline repo reviewability-examples \
  --analyses results/generated/reviewability-examples-gate/analyses/lobsters-rails-migrations,results/generated/reviewability-examples-gate/analyses/forem-rails-migrations \
  --out results/generated/reviewability-examples-gate/examples \
  --json
```

The JSON report writes `reviewability-examples.json` with:

- `examples[]`: generated artifacts with `generated_path`, `generated_kind`, `reviewability_gain`, `non_repair_claim`, `deterministic_outcome`, `proof_holes`, `content_excerpt`, `maintainer_action`, and `required_reanalysis`.
- `summary`: counts for analyses, public repos, examples, `test_examples`, `guard_examples`, accepted-for-review artifacts, listed proof holes, `no_full_repair_claims`, and passed deterministic checks.
- `corpus`: every public repository slice used as evidence.

The key field is `non_repair_claim`: generated tests and guards can improve reviewability by making assumptions executable or inspectable, but they do not prove production safety, apply a migration, discharge proof holes, or replace maintainer review. Patchline preserves `proof_holes` and `required_reanalysis` beside each generated artifact to keep the boundary clear.

`make reviewability-examples-gate` validates this behavior on pinned public repository slices and requires both generated tests and guards, preserved proof holes, and explicit non-repair claims.
