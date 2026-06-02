#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/release-smoke-gate.json}"
OUT="${2:-results/generated/release-smoke}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.release-smoke-gate/v1" and (.ready_suite|length) >= 1' "$SPEC" > /dev/null

# Release readiness = conjunction of all critical checks; advisory checks never block.
jq '
  def eval:
    (map(select(.critical)) ) as $crit
    | (map(select(.critical and (.passed | not)) | .name)) as $blockers
    | {
        ready: ($crit | all(.[]; .passed)),
        blockers: $blockers,
        critical_total: ($crit | length),
        critical_passed: ($crit | map(select(.passed)) | length)
      };
  {
    version: "patchline.release-smoke/v1",
    ready_suite: (.ready_suite | eval),
    blocked_suite: (.blocked_suite | eval)
  }
' "$SPEC" > "$OUT/release-smoke.json"

{
  echo "# Release-blocking smoke suite"
  echo
  echo "Ready suite release-ready: $(jq -r '.ready_suite.ready' "$OUT/release-smoke.json")"
  echo
  echo "Blocked suite release-ready: $(jq -r '.blocked_suite.ready' "$OUT/release-smoke.json")"
  echo
  echo "Blocked suite blockers: $(jq -rc '.blocked_suite.blockers' "$OUT/release-smoke.json")"
} > "$OUT/release-smoke.md"
cp "$OUT/release-smoke.md" "$OUT/README.md"

echo "release-smoke worker: ready=$(jq -r '.ready_suite.ready' "$OUT/release-smoke.json") blocked_ready=$(jq -r '.blocked_suite.ready' "$OUT/release-smoke.json")"
