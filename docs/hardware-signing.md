# Hardware-backed signing

`patchline hardware-signing` verifies that release, gate, and certificate artifacts are bound to attested hardware-backed signing identities before they are trusted as reproducible Patchline evidence.

```bash
go run ./cmd/patchline hardware-signing \
  --spec examples/hardware-signing.json \
  --root . \
  --out results/generated/hardware-signing \
  --json
```

The verifier hashes the signed artifacts, signatures, public keys, hardware attestations, recovery shares, certificate logs, gate reports, drill evidence, and drill results. It rejects forged artifact hashes, missing signatures, unknown or software-backed signers, unmet signing thresholds, missing offline roots, missing recovery shares, missing certificate/gate evidence, and absent key-rotation, recovery, or revocation drills.

Reproduce the positive run, negative controls, and deterministic report hash with:

```bash
make hardware-signing-gate
```
