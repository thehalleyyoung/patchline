#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/release-smoke-gate.json}"
OUT="${2:-results/generated/release-smoke-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.release-smoke-gate/v1" and (.claim|length) > 200 and (.ready_suite|length) >= 1' "$SPEC" > /dev/null

for phrase in "smoke" "release-blocking" "make release-smoke-gate"; do
  grep -F "$phrase" docs/release-smoke.md README.md > /dev/null
done

bash scripts/release-smoke.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in release-smoke.json release-smoke.md README.md; do
  test -s "$OUT/$output"
done

# Ready suite is release-ready despite a failing advisory check; blocked suite is
# blocked and names exactly the failing critical check.
jq -e '
  .version == "patchline.release-smoke/v1" and
  .ready_suite.ready == true and
  (.ready_suite.blockers | length) == 0 and
  .blocked_suite.ready == false and
  (.blocked_suite.blockers == ["core-gate"])
' "$OUT/release-smoke.json" > /dev/null

jq -n --slurpfile r "$OUT/release-smoke.json" '{
  version: "patchline.release-smoke-gate-results/v1",
  ready: $r[0].ready_suite.ready,
  blocked_ready: $r[0].blocked_suite.ready,
  blockers: $r[0].blocked_suite.blockers,
  verified: true
}' > "$OUT/gate-summary.json"

echo "release-smoke gate passed: advisory failure non-blocking, critical failure blocks release (core-gate)"
