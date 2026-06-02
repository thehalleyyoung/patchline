#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/rc-rehearsal-gate.json}"
OUT="${2:-results/generated/rc-rehearsal-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.rc-rehearsal-gate/v1" and (.claim|length) > 200 and (.stages|length) >= 1' "$SPEC" > /dev/null

for phrase in "release-candidate rehearsal" "blessed" "make rc-rehearsal-gate"; do
  grep -F "$phrase" docs/rc-rehearsal.md README.md > /dev/null
done

bash scripts/rc-rehearsal.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in rc-rehearsal.json rc-rehearsal.md README.md; do
  test -s "$OUT/$output"
done

# All-green rehearsal blesses; injected failing stage blocks at exactly that stage.
jq -e '
  .version == "patchline.rc-rehearsal/v1" and
  .release_candidate.blessed == true and
  .release_candidate.all_passed == true and
  .release_candidate.first_failing_stage == null and
  .negative_control.blessed == false and
  .negative_control.first_failing_stage == "reproducibility-replay"
' "$OUT/rc-rehearsal.json" > /dev/null

jq -n --slurpfile r "$OUT/rc-rehearsal.json" '{
  version: "patchline.rc-rehearsal-gate-results/v1",
  blessed: $r[0].release_candidate.blessed,
  blocked_at: $r[0].negative_control.first_failing_stage,
  verified: true
}' > "$OUT/gate-summary.json"

echo "rc-rehearsal gate passed: all-green candidate blessed, failing stage blocks the release candidate"
