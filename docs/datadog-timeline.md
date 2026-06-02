# Datadog-style incident timeline fixtures (real findings)

This gate reconstructs a **Datadog-style incident timeline** around real Patchline findings so
deploy-to-data-change correlation can be exercised offline, without a live observability
backend.

- **Real findings on a timeline.** A real repository is analyzed and its top data-change
  findings are placed on a single incident timeline anchored at a **deploy marker**. Each
  finding contributes an **APM span**, an error **log** line, and feeds the incident's
  **monitor alert**, every entry tagged with `table:` and `finding:`.
- **Ordering and correlation.** The workflow verifies the timeline is ordered
  `deploy < span < log < alert` and that every correlation resolves back to a real finding id,
  reporting correlation coverage.
- **Offline replay.** The output `incident-timeline.json` is a self-contained fixture
  (deploy marker, APM spans, logs, monitor alert) reviewers can load without external services.

```
make datadog-timeline-gate
```

The gate fails unless enough findings are placed on the timeline, every correlation resolves
to a real finding, the timeline is strictly ordered with all findings preceding the alert, and
the assembled fixture carries every layer with table/finding tags.

Outputs (`results/generated/datadog-timeline/`):

- `incident-timeline.json` — the assembled fixture.
- `apm-spans.jsonl` / `logs.jsonl` / `correlations.jsonl` — raw timeline layers.
- `datadog-timeline.json` / `.md` — correlation and ordering summary.
