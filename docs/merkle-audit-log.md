# Merkle-chained audit log

Patchline keeps a **tamper-evident** audit log over all gate runs as a Merkle chain: each entry's hash folds in the previous entry's hash.

## How it works

The worker recomputes the chained hashes from the recorded entries, verifies they match the stored hashes, and reproduces the same check over a tampered log.

## What the gate proves

- The honest log verifies end to end.
- A tampered entry is caught by a hash mismatch.

## Why it matters

A Merkle-chained log makes the evaluation history append-only and auditable, so results cannot be quietly rewritten.

## Reproduce

```
make merkle-audit-log-gate
```
