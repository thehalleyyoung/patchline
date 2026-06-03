# Reviewer fairness audit

Patchline now audits reviewer burden, false-positive burden, and escalation rates across both teams and ecosystems before treating socio-technical review outcomes as evidence.

## What it checks

`patchline reviewer-fairness-audit` loads a versioned observation spec, hashes the real evidence files used by each review row, and computes team-level and ecosystem-level parity metrics:

- average review-minutes burden ratios;
- false-positive-rate gaps over adjudicated findings;
- escalation-rate gaps over owner handoffs.

The audit fails when any group lacks enough reviews or real evidence, when false-positive rates are undefined, or when a burden, false-positive, or escalation disparity exceeds the declared policy.

This is broader than `reviewer-preference-fairness-gate`: that older gate checks scored reviewer-preference items, while this audit checks observed review outcomes and escalation load.

## Reproduce

```bash
go run ./cmd/patchline reviewer-fairness-audit \
  --spec examples/reviewer-fairness-audit.json \
  --root . \
  --out results/generated/reviewer-fairness-audit \
  --json

make reviewer-fairness-audit-gate
```
