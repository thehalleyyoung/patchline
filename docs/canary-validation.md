# Canary-data validation protocol

Patchline's `canary-validate` command compares pre/post migration invariants on sampled, redacted, production-like snapshots. It is meant for teams that cannot share production data but can export a small replay-store slice after local redaction.

The protocol requires the input spec to declare redaction, production-like sampling, matched-row coverage, and a local redaction salt. Generated JSON and Markdown reports emit only aggregate counts, snapshot hashes, row hashes, and cell hashes; they do not emit sampled values, sampled row identifiers, or the salt.

## What it checks

- Row-count preservation within a declared delta.
- Post-migration `not_null` and `unique` obligations.
- Derived-value invariants such as `external_id == legacy_external_id`.
- Stable business fields that must not change across the migration.
- A bounded write set, so canary rows fail when unexpected columns change.

## Reproduce

```bash
go run ./cmd/patchline canary-validate \
  --spec examples/canary-validation-gate.json \
  --before examples/canary-before-snapshot.json \
  --after examples/canary-after-snapshot-good.json \
  --out results/generated/canary-validation \
  --json

make canary-validation-gate
```

The gate also runs a negative post-migration snapshot that duplicates a supposedly unique value, leaves one target empty, and changes a stable business field. Patchline reports exact hash-only counterexamples for those rows while keeping the redacted sample values out of generated artifacts.
