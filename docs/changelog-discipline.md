# Changelog discipline

Patchline treats the changelog as a reproducibility artifact, not a prose-only release note. Every user-visible entry must include:

| Required field | Meaning |
| --- | --- |
| Feature | Short maintainer-facing feature name. |
| User-visible surface | Command, report, file, or GitHub workflow surface users can run or inspect. |
| Real-repo proof | Public repository, pinned ref, and subpath or public artifact used to prove the feature. |
| Gate | `make` target that validates the proof and fails on drift. |

The source of truth is split intentionally:

- `CHANGELOG.md` is human-readable.
- `examples/changelog-gate.json` is machine-checkable.
- `make changelog-gate` validates that both agree, that referenced gates exist, and that a pinned public proof slice still produces real Patchline findings and generated interventions.

This prevents release notes from claiming user-visible behavior without a durable public reproduction path.
