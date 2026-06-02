# Cost-benefit decision model

Patchline provides a cost-benefit decision model monetizing prevented incidents against reviewer time with intervals. This capability is **cost-benefit decision** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the cost-benefit decision claim cannot pass vacuously.

## Why it matters

It keeps the cost-benefit decision claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make cost-benefit-decision-model-gate
```
