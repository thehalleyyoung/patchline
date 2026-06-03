# Incident-postmortem importer

Patchline's `incident-postmortem-import` command turns public remediation lessons into executable detector regression tests. It imports the historical failure suite, reads source-observation JSONL derived from postmortems and follow-up issues, then proves each lesson still triggers the real detector path with a quiet negative control.

The importer currently covers source-grounded lesson classification, destructive protected-table migration detection, missing snapshot rollback detection, and split-brain conflicting-write evidence detection. It writes JSON/Markdown reports, positive and negative fixtures, and a generated Go test package that can be run against the checked-out analyzer.

## Reproduce

```bash
go run ./cmd/patchline incident-postmortem-import \
  --spec examples/incident-postmortem-import.json \
  --out results/generated/incident-postmortem-importer \
  --json

make incident-postmortem-importer-gate
```

The gate fails unless the imported GitLab 2017 and GitHub 2018 public-postmortem lessons produce checked detector regressions, every generated negative control stays quiet, hashes are deterministic, and the generated Go regression test package passes.
