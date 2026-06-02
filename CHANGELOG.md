# Changelog

Patchline changelog entries are evidence-linked: every user-visible feature must name the command or artifact a maintainer sees, the pinned public proof used to validate it, and the gate that keeps the proof reproducible.

## Unreleased

| Feature | User-visible surface | Real-repo proof | Gate |
| --- | --- | --- | --- |
| Generational-artifact frontier (steps 601--700) | 100 new `make <name>-gate` checks across mechanized end-to-end verification, exascale evidence, field causal inference, autonomous fleet, standardization, frontier research, and definitive-artifact 2.0 | `thehalleyyoung/patchline`, self-data, each gate emitting a positive proof and a frozen-spec negative control | `make grand-unified-evidence-index-2-gate` |
| Compatibility gate | `make compatibility-gate` | `lobsters/lobsters@3b80b47aa5aaba37ec44413e7d1dc96fcf1585b6`, `db/migrate`, proving macOS/Linux builds plus a real analysis slice | `make compatibility-gate` |
| Issue triage labels and templates | `.github/ISSUE_TEMPLATE/*.yml` and `.github/labels.yml` | `bytebase/bytebase@0765652ea2dbdf8e93ae44bff5acafc1b97a92cc`, `backend/migrator/migration`, generating sample false-positive, false-negative, parser, ecosystem, and artifact-regression payloads | `make issue-template-gate` |
| Contributor check command | `patchline contributor check` | `thehalleyyoung/patchline`, local worktree, with focused command-package tests and the fast impact gate in one contributor report | `make contributor-check-gate` |
| Structured analysis diagnostics | `patchline repo analyze --trace` | `forem/forem@9c5509c3aeecd4a86a8950206fa937ebcbc2f8d1`, `db/migrate`, plus Bytebase, Mastodon, and Lobsters pinned migration slices emitting successful JSONL spans and summaries | `make diagnostics-gate` |
| Performance budgets | `make performance-budget-gate` | `mastodon/mastodon@facb552c9cdbe8a2ebff0b94ebf2c9e9ec385347`, `db/migrate`, plus large-repo, monorepo, generated-bundle, and four-repo matrix budgets | `make performance-budget-gate` |

## Changelog discipline

Before adding a release note for a user-visible feature:

1. Add or update a row in this file.
2. Add a matching entry to `examples/changelog-gate.json`.
3. Include a public proof repository, pinned ref, subpath when applicable, and gate command.
4. Run `make changelog-gate`.
