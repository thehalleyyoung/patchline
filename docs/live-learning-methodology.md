# Live-learning methodology report

Patchline's **live-learning methodology report** summarizes whether operational
learning improves recall without increasing reviewer over-reliance.

## Contract

```bash
go run ./cmd/patchline feedback methodology-report \
  --spec examples/live-learning-methodology-gate.json \
  --out results/generated/live-learning-methodology-gate --json
```

Every experiment reports recall, over-reliance, and burden deltas and links the
claim to gate-backed evidence. The report fails if recall does not improve or
if over-reliance increases.

## Reproduce

```bash
make live-learning-methodology-gate
```
