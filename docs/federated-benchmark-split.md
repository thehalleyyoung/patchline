# Federated benchmark split

Patchline can run adopter-private benchmark cases locally and publish only
**signed aggregate metrics**. The local split records private case IDs and a
salted partition commitment; the public aggregate keeps only counts, thresholded
metric buckets, manifest/split commitments, and a Patchline signed attestation.

## Local evaluation, public aggregate

```bash
go run ./cmd/patchline artifact-benchmark federated-split \
  --manifest benchmarks/manifests/smoke.json \
  --out results/generated/federated-benchmark-split/split.json \
  --adopter-id adopter-alpha \
  --private-case broad-update \
  --private-case destructive-drop \
  --private-case repair-rollback \
  --min-private-cases 3

go run ./cmd/patchline artifact-benchmark federated-run \
  --split results/generated/federated-benchmark-split/split.json \
  --seed-hex "$PATCHLINE_ATTEST_SEED_HEX" \
  --out results/generated/federated-benchmark-split/aggregate.json

go run ./cmd/patchline artifact-benchmark federated-verify \
  --report results/generated/federated-benchmark-split/aggregate.json
```

`federated-split` validates the benchmark manifest, partitions its cases, and
refuses private partitions smaller than the configured k-anonymity threshold.
`federated-run` executes the normal benchmark runner locally, projects only the
private cases into aggregate buckets, suppresses buckets below the threshold, and
signs the aggregate payload with the existing Patchline attestation envelope.
`federated-verify` strictly decodes the public aggregate, rejects unexpected
case-level fields, checks the signature, and fails if any published bucket is too
small.

## What the gate proves

- The aggregate contains signed thresholded counts, not private `case_id`,
  `fixture`, `ground_truth`, signal, or hash rows.
- The signature binds the dataset, adopter ID, manifest hash, salted split hash,
  partition commitment, k threshold, and aggregate metric buckets.
- Tampered metrics and aggregates with leaked case metadata are rejected.

## Reproduce

```bash
make federated-benchmark-split-gate
```

