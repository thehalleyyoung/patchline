# Rejected generated-code examples

`patchline repo rejected-generated` reads `repo analyze` outputs and produces examples where generated code looks useful in a normal code-review diff but is rejected by deterministic Patchline checks. This is aimed at maintainer review and research evaluation: the report shows the generated artifact, why it appears helpful, the exact deterministic rejection, and the next action.

```bash
go run ./cmd/patchline repo rejected-generated \
  --analyses results/generated/rejected-generated-gate/analyses/lobsters-rails-migrations,results/generated/rejected-generated-gate/analyses/forem-rails-migrations \
  --out results/generated/rejected-generated-gate/rejected \
  --json
```

The JSON report writes `rejected-generated.json` with:

- `examples[]`: rejected generated artifacts with `generated_path`, `generated_kind`, `looks_useful_because`, `normal_diff_appearance`, `deterministic_rejection`, `rejected_status`, `review_badge`, `failed_findings`, `content_excerpt`, `required_next_actions`, and `maintainer_action`.
- `summary`: counts for analyses, public repos, examples, `rejected_interventions`, `plausible_diffs`, `deterministic_rejections`, `high_risk_generated_sql`, failed generated checks, and quarantined generated code.
- `corpus`: every public repository slice used as evidence.

Patchline now scans all generated artifacts for high-risk SQL, not just known template kinds. A generated Markdown or prose artifact can still be rejected if it embeds a plausible-looking mutation such as a broad `UPDATE` or `DELETE`. That makes the report closer to normal code review: the artifact may look like a useful repair suggestion, but deterministic re-analysis blocks it before it becomes trusted work.

`make rejected-generated-gate` validates the command on pinned public repository slices using a deterministic local generator that emits plausible unsafe SQL. The gate requires rejected interventions, concrete failed findings, content excerpts, and maintainer-facing actions.
