# External artifact audit 2.0

Patchline provides an external artifact audit dry run with a reviewer's independent reproduction log. This capability is **external artifact audit** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed item is scored and references concrete backing evidence drawn from existing Patchline gates.

## What the gate proves

- Every item in the spec is scored and carries non-empty backing evidence.
- An unsupported item with empty evidence is rejected, so the external artifact audit claim cannot pass vacuously.

## Why it matters

It keeps the external artifact audit claim honest: the assertion only counts when it is reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make external-artifact-audit-2-gate
```
