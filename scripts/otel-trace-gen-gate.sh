#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/otel-trace-gen-gate.json}"
OUT="${2:-results/generated/otel-trace-gen-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.otel-trace-gen-gate/v1" and (.max_traces | numbers)' "$SPEC" > /dev/null

for phrase in "OpenTelemetry trace" "OTLP" "Expected finding links" "db.sql.table" "make otel-trace-gen-gate"; do
  grep -F "$phrase" docs/otel-trace-gen.md README.md > /dev/null
done

bash scripts/otel-trace-gen.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in otel-trace-gen.json otel-trace-gen.md otlp-traces.json spans.jsonl links.jsonl README.md; do
  test -s "$OUT/$output"
done

min_traces="$(jq '.minimum_traces' "$SPEC")"
min_cov="$(jq '.minimum_link_coverage' "$SPEC")"

jq -e --argjson min_traces "$min_traces" --argjson min_cov "$min_cov" '
  .version == "patchline.otel-trace-gen/v1" and
  .data_change_spans >= $min_traces and
  .links_total == .data_change_spans and
  .link_coverage >= $min_cov and
  .high_findings_traced >= 1
' "$OUT/otel-trace-gen.json" > /dev/null

# OTLP structure must be well-formed and span ids/trace ids hex of correct length.
jq -e '
  (.resourceSpans | length) == 1 and
  (.resourceSpans[0].scopeSpans[0].spans | length) >= 2 and
  all(.resourceSpans[0].scopeSpans[0].spans[];
    (.traceId | test("^[0-9a-f]{32}$")) and
    (.spanId | test("^[0-9a-f]{16}$")) and
    (.name | length) > 0)
' "$OUT/otlp-traces.json" > /dev/null

# Every data-change span must carry a db.sql.table and a patchline.finding_id attribute.
jq -e '
  [ .resourceSpans[0].scopeSpans[0].spans[]
    | select(any(.attributes[]; .key=="patchline.span_role" and .value.stringValue=="data-change")) ] as $dc |
  ($dc | length) >= 1 and
  all($dc[]; (any(.attributes[]; .key=="db.sql.table")) and (any(.attributes[]; .key=="patchline.finding_id")))
' "$OUT/otlp-traces.json" > /dev/null

jq -n --slurpfile r "$OUT/otel-trace-gen.json" '{
  version: "patchline.otel-trace-gen-gate-results/v1",
  data_change_spans: $r[0].data_change_spans,
  link_coverage: $r[0].link_coverage,
  high_findings_traced: $r[0].high_findings_traced,
  verified: true
}' > "$OUT/gate-summary.json"

echo "otel trace gen gate passed: data-change spans $(jq '.data_change_spans' "$OUT/gate-summary.json"), link coverage $(jq '.link_coverage' "$OUT/gate-summary.json"), high traced $(jq '.high_findings_traced' "$OUT/gate-summary.json")"
