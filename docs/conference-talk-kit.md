# Conference-talk and tutorial kit

Patchline provides a conference-talk and tutorial kit with runnable, gate-backed **live demo**s.

## How it works

The worker checks each demo segment maps to a gate command and that all segments are backed.

## What the gate proves

- Every live demo is gate-backed and runnable.
- A segment with no backing gate is rejected.

## Why it matters

Gate-backed live demos never fail on stage and let the audience reproduce everything afterward.

## Reproduce

```
make conference-talk-kit-gate
```
