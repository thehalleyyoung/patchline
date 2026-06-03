# Public evidence marketplace

Patchline's **evidence marketplace** lets organizations publish redacted,
certificate-backed hazard examples without trusting submitter labels or exposing
private source. The publisher accepts an example only when automated release
checks prove its license is clear, its consent statement names the submitter,
grants publication, and references the public license, all listed artifacts are
redacted and content-addressed, and its certificate hash recomputes from the
normalized source, artifact, redaction, license, obligation, and reproduction
metadata.
Every accepted example also receives exact and near-duplicate fingerprints, so
multiple submissions remain auditable without inflating prevalence studies.
When an example supplies community-gate reputation, Patchline scores it only
from reproducibility, longevity, and independent confirmation signals.

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

The reputation score is deterministic and additive. It does not affect the
certificate subject hash because reputation is expected to evolve as gates are
rerun and independently confirmed; the publication report hash still makes the
current reputation snapshot tamper-evident.

## Duplicate-safe prevalence

Publication keeps duplicate submissions visible, but prevalence metrics use
`duplicate_analysis.prevalence_weight`. Exact duplicates share the same
source, hazard metadata, and redacted hazard-artifact hash under different
example IDs. Near duplicates share the same source repository/subpath and
normalized redacted evidence cue even when their artifact bytes or pinned
commits differ. Reports therefore include raw `by_hazard` counts and
duplicate-collapsed `by_hazard_prevalence` counts, plus explicit duplicate
groups and `duplicate_inflation = published - prevalence_examples`.

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
Non-representative duplicate examples are listed under `deduplicated` and are
not imported as additional benchmark cases, preventing prevalence inflation.

## Admission contract

Each public example must include:

| Requirement | Gate behavior |
| --- | --- |
| Clear license | SPDX license must be one of the accepted public licenses, and the per-example `release_admission` report must mark it accepted. |
| Consent | Consent must name the submitting organization, grant publication, and reference the declared public license before `public_release_eligible` can become true. |
| Redaction review | `redaction_reviewed` must be true and `raw_data_shared` false. |
| Artifact hashes | Every relative artifact path is resolved through symlink-safe bounds checks and SHA-256 verified. |
| Certificate backing | Required obligations are `redaction-reviewed`, `license-cleared`, `artifact-hashes-verified`, and `reproducible-without-private-data`. |
| Reproduction | Commands must be public-data-only and free of high-signal credential markers. |
| Gate reputation | Optional `gate_reputation` may contain only `reproducible_runs`, `first_verified_at`, `last_verified_at`, and `independent_confirmations`; malformed timestamps, self-confirmations, duplicate confirmations, negative runs, and private markers are rejected. |
| Duplicate analysis | Every accepted example receives exact and near fingerprints; prevalence counts and benchmark imports use only the representative example in each near-duplicate group. |

## Gate reputation model

Scores are integer-only and computed from registry-supplied timestamps, never
wall-clock time:

| Dimension | Points |
| --- | ---: |
| Reproducible runs | `min(40, runs * 4)` |
| Longevity | `min(30, floor(verified_days / 30) * 5)` |
| Independent confirmations | `min(30, confirmations * 10)` |

Scores below 50 are `emerging`, 50-74 are `reviewable`, and 75 or above are
`established`.

## Reproduce

```bash
make evidence-marketplace-gate
```

The gate publishes the fixture marketplace, checks the duplicate-collapsed
prevalence contract, imports representative examples into a runnable benchmark
manifest, validates and runs that manifest, then corrupts a copied certificate
hash and proves the bad submission is rejected without modifying tracked
fixtures.
