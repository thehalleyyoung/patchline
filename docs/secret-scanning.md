# Secret scanning gate

`make secret-scan-gate` creates a local data-change fixture with deterministic canary values that look like sensitive identifiers, literals, and customer contact data. It then runs:

```bash
go run ./cmd/patchline repo analyze <fixture> \
  --stages inventory,baseline,propose,compare \
  --proposal-kind all \
  --no-llm \
  --redact \
  --trace \
  --ci
```

The gate proves that redacted reports, prompts, generated proposal files, analysis bundles, diagnostics logs, CI outputs, and `redacted-artifacts/` copies do not contain the canaries. It also runs a pinned public Lobsters analysis to keep the gate tied to real repository behavior.

When `repo analyze --redact` is enabled, Patchline writes `redacted-artifacts/` in addition to `analysis-bundle/`. That directory contains stable-token redacted copies of reports, prompt context, prompt text, generated proposal artifacts, compare output, maintainer triage, diagnostics logs, CI output, and bundle artifacts.
