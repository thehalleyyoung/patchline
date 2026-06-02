#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/reviewer-sim-gate.json}"
OUT="${2:-results/generated/reviewer-sim-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.reviewer-sim-gate/v1" and (.claim|length) > 200 and (.reviewers|length) >= 1' "$SPEC" > /dev/null

for phrase in "reviewer panel" "veto" "make reviewer-sim-gate"; do
  grep -F "$phrase" docs/reviewer-sim.md README.md > /dev/null
done

bash scripts/reviewer-sim.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in reviewer-sim.json reviewer-sim.md README.md; do
  test -s "$OUT/$output"
done

# Borderline migration: approved by majority but blocked under veto (strict dissents).
# Clearly-safe migration: approved unanimously under both policies.
jq -e '
  .version == "patchline.reviewer-sim/v1" and
  ([.panels[] | select(.migration=="borderline")][0] | .majority == "approve" and .veto == "block" and .block_votes == 1) and
  ([.panels[] | select(.migration=="clearly_safe")][0] | .majority == "approve" and .veto == "approve" and .block_votes == 0)
' "$OUT/reviewer-sim.json" > /dev/null

jq -n --slurpfile r "$OUT/reviewer-sim.json" '{
  version: "patchline.reviewer-sim-gate-results/v1",
  panels: [$r[0].panels[] | {migration, majority, veto}],
  verified: true
}' > "$OUT/gate-summary.json"

echo "reviewer-sim gate passed: policy changes outcome on borderline migration, unanimous on safe"
