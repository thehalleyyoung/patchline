# Expected benchmark outputs

This directory contains frozen expected reports for executable artifact benchmark manifests.

Compare generated reports with:

```bash
make artifact-benchmark-compare
```

Generated reports are written to `results/generated/artifact-benchmark/`; the committed JSON files here are the golden comparison targets for the smoke and negative manifests.

When a semantic change intentionally changes the expected report shape or result hashes, refresh the committed golden files with:

```bash
make artifact-benchmark-refresh
```

Do not use refresh as a reviewer check: it rewrites this directory. Use `make artifact-benchmark-compare` to detect drift.
