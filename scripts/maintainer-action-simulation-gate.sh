#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/maintainer-action-simulation-gate.json}"
OUT="${2:-results/generated/maintainer-action-simulation-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '
  .version == "patchline.maintainer-action-simulation-gate/v1" and
  (.required_labels | length) == 5
' "$SPEC" > /dev/null

for phrase in "Maintainer-action simulation" "needs-runtime-evidence" "accept/revise/reject" "make maintainer-action-simulation-gate"; do
  grep -F "$phrase" docs/maintainer-action-simulation.md README.md > /dev/null
done

bash scripts/maintainer-action-simulation.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in maintainer-action-simulation.json maintainer-action-simulation.md findings.jsonl README.md; do
  test -s "$OUT/$output"
done

min_findings="$(jq '.minimum_findings' "$SPEC")"

jq -e --argjson min_findings "$min_findings" '
  .version == "patchline.maintainer-action-simulation/v1" and
  .summary.findings >= $min_findings and
  .summary.labels_present == 5 and
  .summary.all_labels_present == true and
  .summary.verified == true and
  (.label_distribution | length) == 5 and
  all(.label_distribution[]; .count >= 1)
' "$OUT/maintainer-action-simulation.json" > /dev/null

# Each required label must be present and rendered.
while read -r lbl; do
  jq -e --arg l "$lbl" 'any(.label_distribution[]; .label == $l and .count > 0)' "$OUT/maintainer-action-simulation.json" > /dev/null
  grep -F "$lbl" "$OUT/maintainer-action-simulation.md" > /dev/null
done < <(jq -r '.required_labels[]' "$SPEC")

# Every finding row must carry a valid decision and reason.
test "$(wc -l < "$OUT/findings.jsonl" | tr -d ' ')" -ge "$min_findings"
jq -e -s 'all(.[]; (.decision | IN("accept","revise","reject","defer","needs-runtime-evidence")) and (.reason | length) > 10)' "$OUT/findings.jsonl" > /dev/null

jq -n \
  --slurpfile report "$OUT/maintainer-action-simulation.json" \
  '{
    version: "patchline.maintainer-action-simulation-gate-results/v1",
    findings: $report[0].summary.findings,
    repositories: $report[0].summary.repositories,
    labels_present: $report[0].summary.labels_present,
    verified: true
  }' > "$OUT/gate-summary.json"

echo "maintainer-action simulation gate passed: findings $(jq '.findings' "$OUT/gate-summary.json"), repositories $(jq '.repositories' "$OUT/gate-summary.json"), labels present $(jq '.labels_present' "$OUT/gate-summary.json")/5"
