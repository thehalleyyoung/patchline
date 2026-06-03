# Resource-adaptive analysis profiles

`patchline resource-profiles` verifies deterministic analysis profiles for constrained laptops, CI runners, air-gapped servers, and public-good hosted tiers.

```bash
go run ./cmd/patchline resource-profiles \
  --spec examples/resource-profiles.json \
  --root . \
  --out results/generated/resource-profiles \
  --json
```

The verifier checks each tier has explicit CPU, memory, time, cost, native-test, network, cache, and degradation policy; every profile must cite hash-backed evidence and map to concrete `patchline` command plans with bounded `files`, `lines`, `tokens`, and `changes`.

Reproduce the positive run, negative controls, and deterministic report hash with:

```bash
make resource-profiles-gate
```
