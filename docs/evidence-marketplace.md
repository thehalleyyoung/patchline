# Public evidence marketplace

Patchline's **evidence marketplace** lets organizations publish redacted,
certificate-backed hazard examples without trusting submitter labels or exposing
private source. The publisher accepts an example only when its license is clear,
its consent statement is explicit, all listed artifacts are redacted and
content-addressed, and its certificate hash recomputes from the normalized
source, artifact, redaction, license, obligation, and reproduction metadata.

## Publish

```bash
go run ./cmd/patchline evidence-marketplace publish \
  --registry examples/evidence-marketplace/registry.json \
  --out results/generated/evidence-marketplace \
  --json
```

The command writes `marketplace.json`, `marketplace.md`, and `index.html`.
Rejected examples stay in the report with reason codes; they are not silently
dropped.

## Import into benchmarks

```bash
go run ./cmd/patchline artifact-benchmark import-marketplace \
  --registry examples/evidence-marketplace/registry.json \
  --out results/generated/marketplace-benchmark \
  --json
go run ./cmd/patchline artifact-benchmark run \
  results/generated/marketplace-benchmark/manifests/marketplace-import.json
```

The importer first reuses marketplace publication checks, then rehashes the
artifact it reads. It records registry and artifact `hazard_class` values as
untrusted submitter labels, derives benchmark labels only from a closed table of
redacted evidence cues, and emits runnable SQL fixtures plus ground truth that
preserves source, certificate, evidence-hash, and artifact-hash provenance.

## Admission contract

Each public example must include:

| Requirement | Gate behavior |
| --- | --- |
| Clear license | SPDX license must be one of the accepted public licenses. |
| Redaction review | `redaction_reviewed` must be true and `raw_data_shared` false. |
| Artifact hashes | Every relative artifact path is resolved through symlink-safe bounds checks and SHA-256 verified. |
| Certificate backing | Required obligations are `redaction-reviewed`, `license-cleared`, `artifact-hashes-verified`, and `reproducible-without-private-data`. |
| Reproduction | Commands must be public-data-only and free of high-signal credential markers. |

## Reproduce

```bash
make evidence-marketplace-gate
```

The gate publishes the fixture marketplace, imports the published examples into
a runnable benchmark manifest, validates and runs that manifest, then corrupts a
copied certificate hash and proves the bad submission is rejected without
modifying tracked fixtures.
