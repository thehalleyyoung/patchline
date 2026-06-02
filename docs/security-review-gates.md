# Security review gates

Patchline protects four merge-sensitive surfaces: `adapters`, `generators`, `archive-handlers`, and `execution-features`. Any pull request that changes one of those surfaces must pass the matching proof gates before merge.

Run:

```bash
patchline security review \
  --changed-files internal/evidence/adapter.go,internal/project/propose.go \
  --passed-gates threat-model-gate,offline-validation-gate,secret-scan-gate,generated-code-quarantine-gate,prompt-context-gate,redaction-stability-gate \
  --out results/generated/security-review \
  --json
```

The command writes `security-review.json` and `security-review.md`. It classifies changed files into protected surfaces, lists required gates, records passed and missing gates, and exits non-zero when a protected surface lacks a required proof gate.

Required gate matrix:

| Surface | Examples | Required gates |
| --- | --- | --- |
| adapters | `internal/evidence/`, adapter docs/examples/scripts | `threat-model-gate`, `offline-validation-gate`, `secret-scan-gate` |
| generators | `internal/project/propose.go`, generated-artifact docs/examples/scripts | `generated-code-quarantine-gate`, `prompt-context-gate`, `redaction-stability-gate` |
| archive handlers | `internal/archive/`, archive extraction/fetch code, archive docs/examples/scripts | `archive-security-gate`, `threat-model-gate`, `offline-validation-gate` |
| execution features | native-test compare code, DB dry-run code, native/dry-run docs/scripts | `generated-code-quarantine-gate`, `db-dry-run-gate`, `threat-model-gate` |

`make security-review-gate` proves the classifier, the non-zero blocking behavior, the passing behavior, documentation coverage, and a pinned public-repository analysis so the gate is anchored in real Patchline artifacts.
