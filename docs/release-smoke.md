# Release-blocking smoke suite

Patchline gates every release behind a minimal **smoke** suite so that a release is
blocked the moment any critical check fails, independent of how many non-critical
checks pass. This is the **release-blocking** safety net.

## Model

Each smoke check is marked `critical` or advisory. Release readiness is the conjunction
of all critical checks; advisory checks are reported but never block.

## Why it stays honest

A release must not ship because most checks were green. The gate proves a suite whose
critical checks all pass is release-ready even with a failing advisory check, and that
a negative-control suite with a single failing critical check is blocked and names that
check (`core-gate`) as the blocker.

## Reproduce

```
make release-smoke-gate
```
