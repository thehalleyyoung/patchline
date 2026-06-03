# Explainability audit

Patchline now gate-checks whether independent reviewers agree that evidence trails support the verdicts Patchline wants maintainers, paper reviewers, or certificate consumers to trust.

## What it checks

`patchline explainability-audit` loads a versioned review spec, hashes the cited evidence files, groups judgments by verdict and reviewer, and fails when:

- too few independent reviewers or verdicts are covered;
- a verdict lacks the required number of reviews;
- reviewers disagree below the declared agreement threshold;
- the supported-review rate is too low or the unsupported-review rate is too high;
- evidence paths are missing, escape the audit root, point at non-regular files, or do not match optional expected SHA-256 hashes.

`supported_rate` is the fraction of all reviews judged `supported`; `unsupported_rate` is the fraction judged `unsupported`; `partial` reviews count in the denominator and must explain which evidence is missing.

## Reproduce

```bash
go run ./cmd/patchline explainability-audit \
  --spec examples/explainability-audit.json \
  --root . \
  --out results/generated/explainability-audit \
  --json

make explainability-audit-gate
```
