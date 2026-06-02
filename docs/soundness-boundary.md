# Soundness-boundary specification

Patchline ships an explicit **soundness boundary** specification enumerating which hazard classes it guarantees to catch, best-effort detects, or treats as out of scope.

## How it works

The worker checks that every hazard class carries an explicit guarantee level and that each guaranteed class is backed by at least one gate.

## What the gate proves

- The boundary is total over the declared hazard classes.
- A guaranteed class with no backing gate is rejected.

## Why it matters

Honest scoping — guaranteed vs best-effort vs out-of-scope, each gate-backed — is what makes a soundness claim credible.

## Reproduce

```
make soundness-boundary-gate
```
