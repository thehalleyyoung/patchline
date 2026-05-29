# Expected benchmark outputs

This directory contains frozen expected reports for executable artifact benchmark manifests.

Compare generated reports with:

```bash
make artifact-benchmark-compare
```

Generated reports are written to `results/generated/artifact-benchmark/`; the committed JSON files here are the golden comparison targets for the smoke, negative, pinned public migration, and offline public incident manifests.

When a semantic change intentionally changes the expected report shape or result hashes, refresh the committed golden files with:

```bash
make artifact-benchmark-refresh
```

Do not use refresh as a reviewer check: it rewrites this directory. Use `make artifact-benchmark-compare` for the offline smoke/negative drift check, `make artifact-benchmark-public` for the explicit network-backed migration drift check, and `make artifact-benchmark-public-incidents` for the offline source-derived incident drift check.
