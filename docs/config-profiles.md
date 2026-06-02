# Configuration profiles

Patchline offers **strict/balanced/lenient** configuration profiles with documented trade-offs, where stricter profiles monotonically increase recall at the cost of precision.

## How it works

The worker checks the three profiles are ordered by recall, that precision trades off in the opposite direction, and that each documents its threshold.

## What the gate proves

- The recall ordering strict >= balanced >= lenient holds.
- A misconfigured profile violating the monotonic trade-off is rejected.

## Why it matters

Named profiles with a documented, gate-checked trade-off let teams choose an operating point instead of guessing thresholds.

## Reproduce

```
make config-profiles-gate
```
