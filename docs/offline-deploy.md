# Reproducible edge/offline deployment

`patchline offline-deploy` verifies regulated edge deployments where operators require no network, no telemetry, and pinned update bundles before a Patchline release can enter an offline site.

```bash
go run ./cmd/patchline offline-deploy \
  --spec examples/offline-deploy.json \
  --root . \
  --out results/generated/offline-deploy \
  --json
```

The verifier hashes every install bundle, signature, SBOM, update bundle, update manifest, rollback artifact, and cited evidence. It rejects profiles with egress endpoints, telemetry destinations, network-fetching commands, missing signatures/SBOMs, unpinned or mismatched bundle hashes, non-offline updates, or rollback plans that lack a prior bundle pin and local command.

Reproduce the positive run, negative controls, and deterministic report hash with:

```bash
make offline-deploy-gate
```
