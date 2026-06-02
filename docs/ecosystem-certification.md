# Extension ecosystem certification

Patchline issues an extension ecosystem certification mark backed by an automated **conformance** gate.

## How it works

The worker runs the conformance checks against each extension and certifies only those passing every check.

## What the gate proves

- A conforming extension earns certification.
- A non-conforming extension is denied the mark.

## Why it matters

A certification mark with a real conformance gate lets users trust third-party extensions.

## Reproduce

```
make ecosystem-certification-gate
```
