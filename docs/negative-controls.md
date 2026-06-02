# Runtime-evidence negative controls

Confirmation is only meaningful if non-confirmation is possible. This gate runs **negative
controls** — the inverse of confirmation — to prove the runtime layer is not a rubber stamp.

For each real finding the workflow runs a paired test:

- **positive arm** — telemetry shows impact (errors, elevated latency) → the static warning is
  *runtime-confirmed*.
- **negative control** — telemetry is silent (zero errors, healthy latency) → the warning must
  stay *unconfirmed*.

The decisive check: **high-severity findings under a silent negative control must NOT be
confirmed**. Confirmation depends only on observed telemetry, never on static severity, so:

1. **Specificity = 1.0** — every silent control is left unconfirmed (zero false confirmations).
2. **Power retained** — the positive arm still confirms, so the test discriminates.

```
make negative-controls-gate
```

Outputs land in `results/generated/negative-controls/`.
