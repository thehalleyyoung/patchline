# Comparison pages

Patchline comparison pages explain how Patchline fits alongside adjacent tooling rather than claiming to replace it. `scripts/generate-comparison-pages.sh` regenerates the pages from pinned public repositories and emits shared evidence for every comparison.

```bash
make comparison-pages-gate
```

The generated page set covers:

- code scanning
- SQL linters
- migration tools
- observability dashboards
- AI coding assistants

Each page includes adjacent-tool strengths, Patchline's data-change repair focus, a complementary workflow, and the same regenerated evidence table: files scanned, ranked risks, provenance slices, generated review artifacts, CI/SARIF outputs, and analysis-bundle files.
