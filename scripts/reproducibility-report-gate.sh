#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/reproducibility-report-gate.json}"
OUT="${2:-results/generated/reproducibility-report-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '
  .version == "patchline.reproducibility-report-gate/v1" and
  (.gates | length) >= .minimum_gates and
  (.required_sections | length) >= 5
' "$SPEC" > /dev/null

for phrase in "monthly reproducibility reports" "cache status" "failures" "fixes" "benchmark trends" "make reproducibility-report-gate"; do
  grep -F "$phrase" docs/reproducibility-reports.md README.md > /dev/null
done

bash scripts/generate-reproducibility-report.sh "$SPEC" "$OUT" > "$OUT.run.log"

min_gates="$(jq '.minimum_gates' "$SPEC")"
min_repos="$(jq '.minimum_public_repos' "$SPEC")"
jq -e --argjson min_gates "$min_gates" --argjson min_repos "$min_repos" '
  .version == "patchline.reproducibility-report/v1" and
  .summary.gates >= $min_gates and
  .summary.passed == .summary.gates and
  .summary.failed == 0 and
  (.summary.public_repos | length) >= $min_repos and
  .summary.cache_files > 0 and
  .summary.verified == true and
  (.summary.trend_deltas | length) == .summary.gates and
  (.summary.fixes | length) == .summary.gates and
  all(.gates[]; .verified == true and .status == "passed" and (.target | startswith("make ")) and (.cache.status | length) > 0)
' "$OUT/reproducibility-report.json" > /dev/null

while read -r section; do
  grep -i -F "$section" "$OUT/report.md" > /dev/null
done < <(jq -r '.required_sections[]' "$SPEC")

while read -r gate_id; do
  test -s "$OUT/gates/$gate_id.run.log"
  test -s "$OUT/gates/$gate_id/report-row.json"
done < <(jq -r '.gates[].id' "$SPEC")

for repo in $(jq -r '.gates[].public_repos[]' "$SPEC" | sort -u); do
  grep -F "$repo" "$OUT/report.md" > /dev/null
done

jq -n \
  --slurpfile report "$OUT/reproducibility-report.json" \
  '{
    version:"patchline.reproducibility-report-gate-results/v1",
    month:$report[0].month,
    gates:$report[0].summary.gates,
    passed:$report[0].summary.passed,
    failed:$report[0].summary.failed,
    public_repos:($report[0].summary.public_repos | length),
    cache_files:$report[0].summary.cache_files,
    verified:true
  }' > "$OUT/gate-summary.json"

echo "reproducibility report gate passed: gates $(jq '.gates' "$OUT/gate-summary.json"), public repos $(jq '.public_repos' "$OUT/gate-summary.json"), cache files $(jq '.cache_files' "$OUT/gate-summary.json")"
