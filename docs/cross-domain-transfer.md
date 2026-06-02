# Transfer to other state-transition domains

Patchline transfers from SQL migrations to other **state-transition** domains (configs, infra-as-code).

## How it works

The worker checks each target domain achieved transfer accuracy above the floor.

## What the gate proves

- Every domain transfers above the accuracy floor.
- A failed transfer is rejected.

## Why it matters

Transfer to configs and IaC shows the hazard framework captures state-transition safety generally.

## Reproduce

```
make cross-domain-transfer-gate
```
