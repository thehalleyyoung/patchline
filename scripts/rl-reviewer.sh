#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/rl-reviewer-gate.json}"; OUT="${2:-results/generated/rl-reviewer}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.rl-reviewer-gate/v1"' "$SPEC" > /dev/null
jq '

  def r4: (.*10000|round)/10000;
  {version:"patchline.rl-reviewer/v1",
   learned_cost:.learned_order_cost,
   random_cost:.random_order_cost,
   improvement:((.random_order_cost - .learned_order_cost) | r4),
   beats_random:(.learned_order_cost < .random_order_cost),
   near_optimal:(.learned_order_cost <= (.optimal_cost * 2)),
   degenerate_beats_random:(.degenerate_order_cost < .random_order_cost)}

' "$SPEC" > "$OUT/out.json"
{ echo "# RL triage-order reviewer"; echo; echo "Learned cost $(jq -r .learned_cost "$OUT/out.json") vs random $(jq -r .random_cost "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "rl-reviewer worker: beats_random=$(jq -r .beats_random "$OUT/out.json")"
