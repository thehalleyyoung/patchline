#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/rl-reviewer-gate.json}"; OUT="${2:-results/generated/rl-reviewer}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.rl-reviewer-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "reviewer cost" "make rl-reviewer-gate"; do grep -F "$phrase" docs/rl-reviewer.md README.md > /dev/null; done
bash scripts/rl-reviewer.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.rl-reviewer/v1" and .beats_random==true and .near_optimal==true and .degenerate_beats_random==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.rl-reviewer-gate-results/v1",learned_cost:$r[0].learned_cost,improvement:$r[0].improvement,beats_random:$r[0].beats_random,verified:true}' > "$OUT/gate-summary.json"
echo "rl-reviewer gate passed: learned triage order beats random, degenerate policy flagged"
