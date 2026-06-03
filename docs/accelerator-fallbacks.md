# Deterministic accelerator-free fallbacks

`patchline accelerator-fallbacks` verifies that every repository-discovered learned component can be reproduced without GPUs, hosted accelerators, or network access.

```bash
go run ./cmd/patchline accelerator-fallbacks \
  --spec examples/accelerator-fallbacks.json \
  --root . \
  --out results/generated/accelerator-fallbacks \
  --json
```

The verifier discovers learned fixtures from `examples/*-gate.json`, requires each discovered component to have a declared CPU fallback, hashes the learned-artifact catalog, fallback implementation, inputs, outputs, and replay evidence, scans fallback code for accelerator or network API references, and checks parity drift against the learned artifact.

Reproduce the positive run, negative controls, and deterministic report hash with:

```bash
make accelerator-fallbacks-gate
```
