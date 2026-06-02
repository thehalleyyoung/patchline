# Per-language finding schema

Patchline runs a separate migration extractor for each ecosystem language — **Python, Ruby,
Go, TypeScript, and Java** — but every extractor emits findings against **one shared
schema**, so a single downstream pipeline consumes results from all languages without
per-language special-casing.

## One contract, five extractors

The worker validates each finding against the required-field contract
(`id, language, table, column, hazard, severity`), reports per-language conformance, and
rejects any finding missing a required field.

## What the gate proves

- All five language extractors produce schema-valid findings.
- A malformed finding lacking the `hazard` field is rejected, and the missing field is named.

## Why it matters

A shared schema means ranking, dedup, triage, and reporting are written once and work for
every language Patchline supports.

## Reproduce

```
make language-extractors-gate
```
