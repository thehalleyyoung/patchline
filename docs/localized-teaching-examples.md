# Localized teaching examples

Patchline checks localized teaching examples as runnable artifacts, not prose promises. Each translated lesson must preserve byte-identical technical tokens such as command names, flags, identifiers, and engine terms; cite real repo evidence; cover required accessibility checks; and include a negative control that proves a bad translation fails.

## Reproduce

```bash
go run ./cmd/patchline localized-teaching-examples \
  --spec examples/localized-teaching-examples.json \
  --root . \
  --out results/generated/localized-teaching-examples \
  --json

make localized-teaching-examples-gate
```

The gate renders JSON and Markdown reports, hashes every cited evidence file, verifies Spanish and French examples for app-developer and DBA audiences, checks technical equivalence by preserving identifiers and `make` commands across locales, and mutates the spec so missing translations, translated identifiers, missing accessibility coverage, absent evidence, and missing negative controls are rejected.
