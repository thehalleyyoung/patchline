# Secret-leak scanner

Patchline runs a **zero-tolerance** secret-leak scanner over all generated artifacts, matching high-entropy and known-pattern secrets.

## How it works

The worker scans every artifact record against the secret patterns, counts matches, and enforces that a clean artifact set has exactly zero leaks.

## What the gate proves

- The legitimate artifact set is leak-free.
- An artifact seeded with a fake API key is caught.

## Why it matters

Publishing reproducible artifacts is only safe if a scanner guarantees no credential ever ships with them.

## Reproduce

```
make secret-leak-scanner-gate
```
