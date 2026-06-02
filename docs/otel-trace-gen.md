# Synthetic OpenTelemetry trace generation (real findings)

Runtime evidence is only credible if it connects to real findings. This gate makes runtime
observability **testable offline** by generating valid **OpenTelemetry** traces directly from a
real Patchline baseline.

- **Real findings in, OTLP out.** A real repository is analyzed, high-severity findings are
  selected, and each becomes an OTLP span under a shared deploy parent. Spans carry standard
  semantic-convention attributes (`db.system`, `db.operation`, `db.sql.table`, `db.statement`)
  plus `patchline.finding_id` so they round-trip back to the static finding.
- **Expected finding links.** For every data-change span the workflow emits a link record and
  verifies it resolves to a real finding **by id and table**, reporting link coverage and how
  many high-severity findings were traced.
- **Offline replay.** The output is a single OTLP `ExportTraceServiceRequest`
  (`resourceSpans/scopeSpans/spans`) that any OpenTelemetry-compatible viewer or collector can
  ingest without a live backend.

```
make otel-trace-gen-gate
```

The gate fails unless enough data-change spans are generated, every link resolves to a real
finding (coverage meets the floor), the OTLP document is well-formed with hex trace/span ids,
and every data-change span carries `db.sql.table` and `patchline.finding_id`.

Outputs (`results/generated/otel-trace-gen/`):

- `otlp-traces.json` — the replayable OTLP document.
- `spans.jsonl` / `links.jsonl` — raw spans and expected finding links.
- `otel-trace-gen.json` / `.md` — coverage summary.
