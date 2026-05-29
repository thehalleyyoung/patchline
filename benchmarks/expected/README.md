# Expected benchmark outputs

This directory contains frozen expected reports for executable artifact benchmark manifests and expected-hash manifests for artifact study reports.

Compare generated reports with:

```bash
make artifact-benchmark-compare
make artifact-studies-compare
```

Generated benchmark reports are written to `results/generated/artifact-benchmark/`; the committed benchmark JSON files here are the golden comparison targets for the smoke, negative, repair, semantic-regression, pinned public migration, and offline public incident manifests.

Generated study reports are written to `results/generated/artifact-studies/`; `studies-strict.json` and `studies-public-migrations.json` store the stable `hash` values for `baselines.json`, `ablations.json`, and `scale.json`. Study comparison uses those report hashes rather than exact JSON so analyzer/runtime timing fields do not create false drift.

When a semantic change intentionally changes the expected report shape or result hashes, refresh the committed golden files with:

```bash
make artifact-benchmark-refresh
make artifact-studies-refresh
```

Do not use refresh as a reviewer check: it rewrites this directory. Use `make artifact-benchmark-compare` and `make artifact-studies-compare` for offline drift checks, `make artifact-benchmark-public` and `make artifact-studies-public-compare` for explicit network-backed migration drift checks, and `make artifact-benchmark-public-incidents` for the offline source-derived incident drift check. The offline benchmark compare includes `repair-cases-report.json` and `semantic-regressions-report.json` so replay/proof and archive-memory claims are drift-checked by default.
