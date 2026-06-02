# Incident auto-remediation

Patchline provides an incident auto-remediation loop that proposes and verifies a fix before any human is paged. This capability is **auto-remediation** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the auto-remediation claim cannot pass vacuously.

## Why it matters

It keeps the auto-remediation claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make incident-auto-remediation-gate
```
