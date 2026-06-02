# Automated theorem-discovery loop

Patchline ships an automated theorem-discovery loop proposing and proving new safety lemmas, exercising the **theorem-discovery** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/theorem-discovery-loop`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The theorem-discovery claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "theorem-discovery" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make theorem-discovery-loop-gate
```
