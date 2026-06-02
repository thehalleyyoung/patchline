# Certificate-composition lattice

Patchline ships a mechanized proof that gate-certificate composition forms a lattice with a top safe element, exercising the **lattice** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/certificate-lattice-proof`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The lattice claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "lattice" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make certificate-lattice-proof-gate
```
