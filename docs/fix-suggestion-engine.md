# Fix-suggestion engine

Patchline proposes a **safe migration variant** for each detected hazard, mapping every hazard class to a concrete remediation.

## How it works

The worker checks each hazard has a suggested fix whose post-fix verdict is safe, and computes the remediation coverage.

## What the gate proves

- Every hazard receives a verdict-clearing fix.
- A bogus suggestion whose post-fix verdict is still a hazard is rejected.

## Why it matters

Telling a developer what to do — and proving it clears the hazard — is far more valuable than just flagging the problem.

## Reproduce

```
make fix-suggestion-engine-gate
```
