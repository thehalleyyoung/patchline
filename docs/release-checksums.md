# Signed release checksums

Patchline releases should publish a sorted `checksums.sha256` file and an Ed25519 `checksums.attestation.json` for the checksum file.

Use:

```bash
patchline release checksums \
  --subject patchline-release-vX.Y.Z \
  --seed-hex "$PATCHLINE_RELEASE_SEED_HEX" \
  --artifact dist/patchline-darwin-arm64.tar.gz \
  --artifact dist/patchline-linux-amd64.tar.gz \
  --out dist/release-checksums
```

The command writes:

- `checksums.sha256`: sorted SHA-256 lines for every release artifact.
- `checksums.attestation.json`: Ed25519 signature over the checksum file.
- `release-checksums.json`: artifact metadata, verification status, report hash, and reproducible build instructions.

Recommended reproducible build command:

```bash
CGO_ENABLED=0 \
GOFLAGS='-trimpath -buildvcs=false' \
go build -ldflags '-buildid=' -o bin/patchline ./cmd/patchline
```

Use the release commit timestamp as `SOURCE_DATE_EPOCH` when packaging archives, keep file ordering sorted, and avoid embedding local paths or VCS metadata in binaries. Consumers can verify:

```bash
patchline verify-artifact dist/release-checksums/checksums.attestation.json \
  --artifact dist/release-checksums/checksums.sha256 \
  --json
```

Run `make release-checksum-gate` to prove the reproducible build flags, signed checksum generation, attestation verification, and release packaging against a pinned public repository analysis.
