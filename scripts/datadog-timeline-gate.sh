#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/datadog-timeline-gate.json}"
OUT="${2:-results/generated/datadog-timeline-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.datadog-timeline-gate/v1" and (.max_findings | numbers)' "$SPEC" > /dev/null

for phrase in "Datadog-style incident timeline" "deploy marker" "APM span" "monitor alert" "make datadog-timeline-gate"; do
  grep -F "$phrase" docs/datadog-timeline.md README.md > /dev/null
done

bash scripts/datadog-timeline.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in datadog-timeline.json datadog-timeline.md incident-timeline.json apm-spans.jsonl logs.jsonl correlations.jsonl README.md; do
  test -s "$OUT/$output"
done

min_findings="$(jq '.minimum_findings' "$SPEC")"
min_cov="$(jq '.minimum_correlation_coverage' "$SPEC")"

jq -e --argjson min_findings "$min_findings" --argjson min_cov "$min_cov" '
  .version == "patchline.datadog-timeline/v1" and
  .findings_on_timeline >= $min_findings and
  .correlation_coverage >= $min_cov and
  .timeline_ordered == true and
  .all_findings_before_alert == true and
  .deploy_ts < .alert_ts
' "$OUT/datadog-timeline.json" > /dev/null

# The assembled fixture must carry every timeline layer with table/finding tags.
jq -e '
  (.deploy_marker.type == "deploy_marker") and
  (.monitor_alert.type == "monitor_alert") and
  (.apm_spans | length) >= 1 and
  (.logs | length) >= 1 and
  all(.apm_spans[]; (.meta["db.table"] | length) > 0 and (.meta["patchline.finding_id"] | length) > 0) and
  all(.logs[]; (.ddtags | test("table:")) and (.ddtags | test("finding:")))
' "$OUT/incident-timeline.json" > /dev/null

jq -n --slurpfile r "$OUT/datadog-timeline.json" '{
  version: "patchline.datadog-timeline-gate-results/v1",
  findings_on_timeline: $r[0].findings_on_timeline,
  correlation_coverage: $r[0].correlation_coverage,
  timeline_ordered: $r[0].timeline_ordered,
  verified: true
}' > "$OUT/gate-summary.json"

echo "datadog timeline gate passed: findings $(jq '.findings_on_timeline' "$OUT/gate-summary.json"), coverage $(jq '.correlation_coverage' "$OUT/gate-summary.json"), ordered $(jq '.timeline_ordered' "$OUT/gate-summary.json")"
