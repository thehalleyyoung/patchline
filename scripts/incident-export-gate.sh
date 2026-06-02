#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/incident-export-gate.json}"
OUT="${2:-results/generated/incident-export-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.incident-export-gate/v1" and (.adapters|length)==4' "$SPEC" > /dev/null

for phrase in "Incident export adapters" "PagerDuty" "Opsgenie" "Statuspage" "make incident-export-gate"; do
  grep -F "$phrase" docs/incident-export.md README.md > /dev/null
done

bash scripts/incident-export.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in incident-exports.json incident-export.json pagerduty.json opsgenie.json slack.json statuspage.json incident-export.md README.md; do
  test -s "$OUT/$output"
done

minf="$(jq '.minimum_findings' "$SPEC")"

jq -e --argjson minf "$minf" '
  .version == "patchline.incident-export/v1" and
  .incidents_per_adapter >= $minf and
  .pagerduty_valid == true and
  .opsgenie_valid == true and
  .slack_valid == true and
  .statuspage_valid == true and
  .severity_mapped == true and
  .cross_adapter_linkage == true and
  .all_valid == true
' "$OUT/incident-export.json" > /dev/null

# Each split adapter file must be a JSON array with the documented per-adapter count.
for adapter in pagerduty opsgenie slack statuspage; do
  jq -e 'type == "array" and length > 0' "$OUT/$adapter.json" > /dev/null
done

jq -n --slurpfile r "$OUT/incident-export.json" '{
  version: "patchline.incident-export-gate-results/v1",
  incidents_per_adapter: $r[0].incidents_per_adapter,
  all_valid: $r[0].all_valid,
  severity_mapped: $r[0].severity_mapped,
  cross_adapter_linkage: $r[0].cross_adapter_linkage,
  verified: true
}' > "$OUT/gate-summary.json"

echo "incident export gate passed: per-adapter $(jq '.incidents_per_adapter' "$OUT/gate-summary.json"), all_valid $(jq '.all_valid' "$OUT/gate-summary.json")"
