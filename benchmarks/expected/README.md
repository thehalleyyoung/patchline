# Expected benchmark outputs

This directory contains frozen expected reports for executable artifact benchmark manifests.

Regenerate and compare them with:

```bash
make artifact-benchmark-compare
```

Generated reports are written to `results/generated/artifact-benchmark/`; the committed JSON files here are the golden comparison targets for the smoke and negative manifests.
