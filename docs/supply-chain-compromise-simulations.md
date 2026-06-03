# Supply-chain compromise simulations

`patchline supply-chain simulate` replays local, deterministic supply-chain compromise simulations instead of trusting release metadata at face value. The checked-in gate covers dependency poisoning, malicious archives, and forged release metadata with concrete artifact hashes, signer evidence, quarantine decisions, and negative controls.

The simulation report distinguishes the attack signal from the control outcome. A dependency may have a typosquat source, lockfile drift, an unapproved signer, and an unallowlisted transitive dependency; the run is still healthy only when those signals are detected, rejected, quarantined, and cited with readable evidence. Archive simulations model unsafe entries in the manifest rather than materializing traversal or symlink payloads on disk. Release simulations compare expected version/ref/commit, artifact digest, manifest digest, signer, and certificate-log evidence.

Run:

```bash
go run ./cmd/patchline supply-chain simulate \
  --spec examples/supply-chain-compromise-sim.json \
  --root . \
  --out results/generated/supply-chain-compromise-sim \
  --json
```

Reproduce the positive and negative controls with:

```bash
make supply-chain-compromise-sim-gate
```

The gate mutates the fixture so attacks become undetected, unrejected, unquarantined, and missing signature/certificate evidence. A passing gate requires deterministic counterexamples for each compromised dimension.
