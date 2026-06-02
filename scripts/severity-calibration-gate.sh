#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/severity-calibration-gate.json}"
OUT="${2:-results/generated/severity-calibration-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.severity-calibration-gate/v1" and (.required_severities | length) == 3' "$SPEC" > /dev/null

for phrase in "Calibrated severity validation" "danger-corroborated" "calibration lift" "make severity-calibration-gate"; do
  grep -F "$phrase" docs/severity-calibration.md README.md > /dev/null
done

bash scripts/severity-calibration.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in severity-calibration.json severity-calibration.md findings.jsonl README.md; do
  test -s "$OUT/$output"
done

min_findings="$(jq '.minimum_findings' "$SPEC")"
min_lift="$(jq '.minimum_lift' "$SPEC")"

jq -e --argjson min_findings "$min_findings" --argjson min_lift "$min_lift" '
  .version == "patchline.severity-calibration/v1" and
  .summary.findings >= $min_findings and
  .summary.severities_present == 3 and
  .summary.calibrated == true and
  .summary.verified == true and
  .summary.lift >= $min_lift and
  .summary.elevated_danger_rate > .summary.low_danger_rate and
  (.per_severity | length) == 3 and
  all(.per_severity[]; .findings >= 0 and .danger_rate >= 0 and .danger_rate <= 1)
' "$OUT/severity-calibration.json" > /dev/null

# Each required severity must be present in the calibration table and rendered.
while read -r sev; do
  jq -e --arg s "$sev" 'any(.per_severity[]; .severity == $s)' "$OUT/severity-calibration.json" > /dev/null
  grep -F "$sev" "$OUT/severity-calibration.md" > /dev/null
done < <(jq -r '.required_severities[]' "$SPEC")

# In every repository that has low-severity findings, an elevated bucket must out-rate low.
jq -e 'all(.by_repository[]; .low_findings == 0 or ((.high_rate > .low_rate) or (.medium_rate > .low_rate)))' "$OUT/severity-calibration.json" > /dev/null

# Each finding row must carry a real severity and boolean corroboration.
test "$(wc -l < "$OUT/findings.jsonl" | tr -d ' ')" -ge "$min_findings"
jq -e -s 'all(.[]; (.severity | IN("high","medium","low")) and (.corroborated | type == "boolean"))' "$OUT/findings.jsonl" > /dev/null

jq -n \
  --slurpfile report "$OUT/severity-calibration.json" \
  '{
    version: "patchline.severity-calibration-gate-results/v1",
    findings: $report[0].summary.findings,
    elevated_danger_rate: $report[0].summary.elevated_danger_rate,
    low_danger_rate: $report[0].summary.low_danger_rate,
    lift: $report[0].summary.lift,
    verified: true
  }' > "$OUT/gate-summary.json"

echo "severity calibration gate passed: findings $(jq '.findings' "$OUT/gate-summary.json"), elevated rate $(jq '.elevated_danger_rate' "$OUT/gate-summary.json"), low rate $(jq '.low_danger_rate' "$OUT/gate-summary.json"), lift $(jq '.lift' "$OUT/gate-summary.json")"
