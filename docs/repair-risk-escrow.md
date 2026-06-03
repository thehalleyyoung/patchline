# Repair-risk escrow

Patchline's `repair-escrow` command keeps proposed fixes in escrow until they accumulate enough repair-bound evidence to clear manual-review and certificate thresholds. It counts distinct reviewer identities, distinct valid certificate IDs, and accepted evidence items; duplicate approvals do not advance the gate.

## What it enforces

- A repair releases only after every threshold is met.
- Manual rejections, failed evidence, revoked certificates, expired certificates, and artifact-hash mismatches reject the repair even if positive evidence is present.
- Held repairs get quantified obligations such as the remaining manual reviewers or certificates needed.
- Every review, certificate, and evidence item must bind to both the repair ID and proposed artifact hash.

## Reproduce

```bash
go run ./cmd/patchline repair-escrow \
  --spec examples/repair-risk-escrow.json \
  --out results/generated/repair-risk-escrow \
  --json

make repair-risk-escrow-gate
```
