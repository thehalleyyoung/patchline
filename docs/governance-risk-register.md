# Governance-risk register

Patchline now gate-checks whether project control is concentrated across four infrastructure-critical domains: **maintainership**, **funding**, **infrastructure**, and **benchmark control**.

## What it checks

`patchline governance-risk-register` loads a versioned register, hashes every evidence file, and computes per-domain control concentration:

- top owner and top organization share by weighted controlled asset;
- independent owner and organization counts;
- stale review-cadence, missing rotation-plan, and missing evidence failures;
- high-risk domains that lack enough mitigations.

The register uses an explicit `as_of_date`, so review-cadence results and report hashes are reproducible instead of depending on wall-clock time.

## Reproduce

```bash
go run ./cmd/patchline governance-risk-register \
  --spec examples/governance-risk-register.json \
  --root . \
  --out results/generated/governance-risk-register \
  --json

make governance-risk-register-gate
```
