# Release notes

Patchline generates release notes grounded in reproducible evidence rather than
prose. The notes embed three things:

- **public proof deltas** — the set of gates present in this release but absent from
  the previous one, computed by diffing the declared previous-release gate set
  against the gates actually present in `scripts/*-gate.sh` today;
- **contributor recognition** — highlights drawn from gate-backed data; and
- a **known-limitations** ledger.

## Why it stays honest

Because the proof delta is derived from the live scripts directory, the notes can
never advertise a capability whose gate is absent. The gate asserts that the delta
is non-empty, that **every** advertised new proof maps to a real gate script, and
that the recognition and known-limitations sections are present and non-empty.

## Reproduce

```
make release-notes-gate
```
