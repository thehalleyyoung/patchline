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
Publication also writes a long-term **archive mirror**: each accepted artifact is
copied to a content-addressed `archive/sha256/<hash>` path and listed with its
checksum, license, source/certificate hashes, and withdrawal metadata.

## Publish

```bash
go run ./cmd/patchline evidence-marketplace publish \
  --registry examples/evidence-marketplace/registry.json \
  --out results/generated/evidence-marketplace \
  --json
```

The command writes `marketplace.json`, `marketplace.md`, `index.html`,
`archive-mirror.json`, and content-addressed copies under `archive/sha256/`.
Rejected examples stay in the report with reason codes; they are not silently
dropped.

The reputation score is deterministic and additive. It does not affect the
certificate subject hash because reputation is expected to evolve as gates are
rerun and independently confirmed; the publication report hash still makes the
current reputation snapshot tamper-evident.

## Long-term archive mirror

The archive mirror persists every accepted, already-redacted marketplace artifact
even when duplicate analysis later assigns it zero prevalence weight. The mirror
manifest records the artifact checksum, byte count, SPDX license, source commit,
certificate subject hash, evidence hash, and a deterministic withdrawal record
for each artifact. Withdrawal metadata starts in `active` state with a stable
`withdrawal_id`, maintainer contact, policy URL, review requirement, tombstone
requirement, and checksum-preservation requirement so future takedowns can hide
bytes without erasing the historical proof that was reviewed.

`WriteReport` re-reads and rehashes source artifacts before copying them into
the mirror; if bytes drift after publication validation, report writing fails
instead of publishing stale checksums.

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

## Adversarial migration challenge

```bash
go run ./cmd/patchline evidence-marketplace challenge \
  --registry examples/evidence-marketplace/challenge-registry.json \
  --out results/generated/adversarial-challenge \
  --json
```

The public adversarial migration challenge is a separate marketplace track for
redacted, certificate-backed migrations designed to stress detectors without
publishing private exploit context. Its deterministic scoring is
certificate-bound, and
scoreboard entries are sorted deterministically by score, then ID. The score is
computed from verified artifact hashes, the migration analyzer's actual
high-risk result on the public-safe SQL proof, reproduction evidence, duplicate
novelty, minimization against `max_public_proof_lines`, and
responsible-disclosure checks. Submitter-provided labels do not directly grant
scoreboard credit.

### Responsible-disclosure rules

Committed challenge artifacts must be public-safe and redacted. Embargoed or
non-public exploit details can appear only as a `sha256:` reference hash; an
entry with `disclosure.status` other than `public-safe` or
`public_release_allowed=false` is rejected before it can appear on the
scoreboard. Each challenge certificate must include the conditional
`responsible-disclosure-cleared` obligation, and the track must publish contact,
policy URL, embargo length, scoring weights, and the minimum scoreboard score.

## Admission contract

Each public example must include:

| Requirement | Gate behavior |
| --- | --- |
| Clear license | SPDX license must be one of the accepted public licenses, and the per-example `release_admission` report must mark it accepted. |
| Consent | Consent must name the submitting organization, grant publication, and reference the declared public license before `public_release_eligible` can become true. |
| Redaction review | `redaction_reviewed` must be true and `raw_data_shared` false. |
| Artifact hashes | Every relative artifact path is resolved through symlink-safe bounds checks and SHA-256 verified. |
| Certificate backing | Required obligations are `redaction-reviewed`, `license-cleared`, `artifact-hashes-verified`, and `reproducible-without-private-data`. |
| Archive mirror | Accepted artifacts are copied to `archive/sha256/<hash>` and listed with checksum, license, source, certificate, evidence, and withdrawal metadata. |
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
make adversarial-challenge-gate
```

The gate publishes the fixture marketplace, checks the duplicate-collapsed
prevalence contract, imports representative examples into a runnable benchmark
manifest, validates and runs that manifest, then corrupts a copied certificate
hash and proves the bad submission is rejected without modifying tracked
fixtures. The challenge gate scores public-safe adversarial migrations from real
redacted SQL artifacts, verifies analyzer-backed high-risk proofs, and runs the
embargo/publication negative controls in focused tests.
