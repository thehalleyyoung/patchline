# Causal-discovery of incident-causing patterns

Patchline's **causal**-discovery module infers which schema patterns cause incidents from observational data.

## How it works

The worker checks each inferred cause-incident edge carries concrete causal evidence.

## What the gate proves

- Every causal edge is evidence-backed.
- A spurious correlation is rejected.

## Why it matters

Causal attribution avoids blaming patterns that merely co-occur with the real incident driver.

## Reproduce

```
make causal-discovery-module-gate
```
