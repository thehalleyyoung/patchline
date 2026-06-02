# Structured diagnostics

`patchline repo analyze --trace` writes machine-readable diagnostics for long analyses without changing the default command output. The diagnostics are intended for maintainers debugging slow or surprising runs on real repositories.

```bash
go run ./cmd/patchline repo analyze --github bytebase/bytebase \
  --subpath backend/migrator/migration \
  --stages inventory,baseline,propose,compare,deep \
  --budget files=4,lines=80,tokens=12000,changes=2 \
  --no-llm \
  --trace \
  --out results/generated/diagnostics/bytebase
```

When tracing is enabled, Patchline writes:

| Artifact | Purpose |
| --- | --- |
| `diagnostics/events.jsonl` | Ordered JSONL span and log events with trace IDs, span IDs, parent IDs, status, duration, elapsed time, attributes, and errors. |
| `diagnostics/summary.json` | Stable summary with event, span, log, failed-span, duration, and content hash fields. |
| `analyze.json` / `analyze.md` | Include the diagnostics output path and summary counts. |

The trace covers the orchestration stages that usually dominate analysis time: fetch, inventory, intake, baseline, deep summary, proposal generation, compare, maintainer triage, analysis-bundle writing, and CI artifact writing when `--ci` is used. Failed stages emit error spans before returning the underlying error.

Run `make diagnostics-gate` to prove the trace contract against four pinned public repository slices. The gate checks that each run emits diagnostics artifacts, required stage spans, zero failed spans for successful analyses, positive durations, and a normal analysis report with ranked risks and generated interventions.
