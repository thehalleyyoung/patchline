# Automated generator of novel hazards

Patchline's benchmark-generator synthesizes **novel** hazards beyond the existing taxonomy.

## How it works

The worker checks each generated hazard is both novel relative to the taxonomy and a valid migration.

## What the gate proves

- Every generated hazard is novel and valid.
- An invalid or duplicate hazard is rejected.

## Why it matters

A generator of novel valid hazards keeps the benchmark ahead of overfitting to known cases.

## Reproduce

```
make hazard-benchmark-generator-gate
```
