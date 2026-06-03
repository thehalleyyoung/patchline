# Confidential-computing evaluation

`patchline confidential-computing` evaluates private corpus analysis plans with verifiable enclave attestation before their outputs are trusted as Patchline evidence.

```bash
go run ./cmd/patchline confidential-computing \
  --spec examples/confidential-computing.json \
  --root . \
  --out results/generated/confidential-computing \
  --json
```

The verifier hashes attestation quotes, verifier reports, key-release policies, encrypted input manifests, encrypted private-corpus bundles, redacted public aggregates, private retained outputs, and deterministic replay evidence. It rejects missing quotes, wrong measurements, missing verifier reports, plaintext export, missing encrypted inputs, forged output manifests, network egress, unredacted public outputs, policy/enclave mismatches, and missing replay evidence.

Reproduce the positive run, negative controls, and deterministic report hash with:

```bash
make confidential-computing-gate
```
