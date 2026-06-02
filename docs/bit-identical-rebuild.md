# Bit-identical rebuild from frozen snapshot

Patchline guarantees any release rebuilds **bit-identical**ly from its frozen snapshot.

## How it works

The worker checks each release rebuilds to the identical content hash from its snapshot.

## What the gate proves

- Every release is bit-identical on rebuild.
- A non-deterministic build is rejected.

## Why it matters

Bit-identical rebuilds make a release independently verifiable down to the byte.

## Reproduce

```
make bit-identical-rebuild-gate
```
