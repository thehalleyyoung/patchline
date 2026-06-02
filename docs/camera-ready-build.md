# Camera-ready build pipeline

Patchline builds the final PDF from source with **pinned tooling**, so the camera-ready artifact is reproducible.

## How it works

The worker checks the build pins the TeX toolchain version, declares its source inputs, and produces a fixed output name.

## What the gate proves

- The camera-ready build is fully pinned and source-driven.
- A build relying on an unpinned floating tool version is rejected.

## Why it matters

Pinned tooling means the camera-ready PDF rebuilds identically years later, not just on today's laptop.

## Reproduce

```
make camera-ready-build-gate
```
