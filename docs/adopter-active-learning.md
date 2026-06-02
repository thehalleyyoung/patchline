# Adopter-local active-learning queue

Patchline's **adopter-local active-learning queue** ranks uncertain local
examples for reviewer labeling while keeping individual examples inside the
organization.

## Contract

```bash
go run ./cmd/patchline feedback active-learning-queue \
  --spec examples/adopter-active-learning-gate.json \
  --out results/generated/adopter-active-learning-gate --json
```

The local queue is written with `shareable: false` and includes opaque local
case IDs for the adopter. The separate `active-learning-aggregate.json` is the
shareable artifact: it contains detector-level counts, mean uncertainty,
information gain, and burden without local case IDs, finding IDs, source code,
file paths, diffs, evidence hashes, adopter IDs, or salts.

## Reproduce

```bash
make adopter-active-learning-gate
```
