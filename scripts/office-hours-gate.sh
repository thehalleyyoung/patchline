#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/office-hours-gate.json}"
OUT="${2:-results/generated/office-hours-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.office-hours-gate/v1" and (.claim|length) > 200 and (.recent_failures|length) >= 1 and (.roadmap_cards|length) >= 1' "$SPEC" > /dev/null

for phrase in "office hours" "reproducibility" "make office-hours-gate"; do
  grep -F "$phrase" docs/office-hours.md README.md > /dev/null
done

bash scripts/office-hours.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in office-hours.json agenda.md README.md; do
  test -s "$OUT/$output"
done

# Every failure item must reference a real gate; roadmap items must be present.
jq -e '
  .version == "patchline.office-hours/v1" and
  .failure_count >= 1 and
  .roadmap_count >= 1 and
  (.failures | length) == .failure_count
' "$OUT/office-hours.json" > /dev/null

for g in $(jq -r '.failures[]' "$OUT/office-hours.json"); do
  test -f "scripts/${g}.sh"
done

for section in "## Review (reproducibility failures)" "## Triage" "## Planning (roadmap cards)"; do
  grep -Fq "$section" "$OUT/agenda.md"
done

jq -n --slurpfile r "$OUT/office-hours.json" '{
  version: "patchline.office-hours-gate-results/v1",
  session: $r[0].session,
  failure_count: $r[0].failure_count,
  roadmap_count: $r[0].roadmap_count,
  verified: true
}' > "$OUT/gate-summary.json"

echo "office-hours gate passed: agenda driven by real gate failures + roadmap cards, all sections present"
