# DOI-pinned artifact snapshot

Patchline ships a frozen, DOI-pinned artifact snapshot with a published bit-identical rebuild attestation, exercising the **DOI-pinned** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/doi-pinned-artifact-snapshot`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The DOI-pinned claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "DOI-pinned" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make doi-pinned-artifact-snapshot-gate
```
