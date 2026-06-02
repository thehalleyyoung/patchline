# Human-trust regression suite

Patchline's **human-trust regression suite** checks whether explanations remain
faithful after learned components update.

## Contract

```bash
go run ./cmd/patchline feedback trust-regression \
  --spec examples/human-trust-regression-gate.json \
  --out results/generated/human-trust-regression-gate --json
```

The suite compares explanation faithfulness, evidence citation coverage,
uncertainty disclosure, reviewer over-reliance, and review burden against
explicit tolerances. A release fails if explanations hide uncertainty, lose
evidence links, or increase over-reliance beyond the tolerance.

## Reproduce

```bash
make human-trust-regression-gate
```
