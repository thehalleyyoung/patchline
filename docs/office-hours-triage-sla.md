# Office-hours triage SLA

Patchline ships a public office-hours and triage rotation with published response-time SLAs met for a year, exercising the **response-time SLA** property on real Patchline self-data.

## How it works

The worker loads a frozen spec and checks each item is scored and references concrete backing evidence (an existing reproducible gate) under `results/generated/office-hours-triage-sla`.

## What the gate proves

- Every item is scored with concrete, gate-backed evidence.
- An unsupported item with empty evidence is rejected.
- The response-time SLA claim is reproducible from a frozen, content-addressed spec.

## Why it matters

Turning "response-time SLA" into a re-runnable check keeps the claim honest, citable, and regression-guarded.

## Reproduce

```
make office-hours-triage-sla-gate
```
