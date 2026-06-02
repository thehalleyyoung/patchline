# Backed-limitations gate

Patchline ensures every claimed **limitation** has a backing experiment or example.

## How it works

The worker checks each limitation references a real backing artifact and that the reference resolves.

## What the gate proves

- Every limitation is demonstrably backed.
- A speculative limitation with no example is rejected.

## Why it matters

Limitations grounded in demonstrable behavior build reviewer trust; vague caveats erode it.

## Reproduce

```
make limitations-gate-gate
```
