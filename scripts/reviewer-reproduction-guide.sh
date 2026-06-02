#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/reviewer-reproduction-guide-gate.json}"; OUT="${2:-results/generated/reviewer-reproduction-guide}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.reviewer-reproduction-guide-gate/v1"' "$SPEC" > /dev/null
jq '

  .steps as $S
  | {version:"patchline.reviewer-reproduction-guide/v1",
     steps:($S|length),
     within_step_budget:(($S|length) <= .max_steps),
     within_time_budget:(.runtime_minutes <= .max_minutes),
     reaches_headline:.reaches_headline,
     bloated_within_budget:(.bloated_steps_count <= .max_steps)}

' "$SPEC" > "$OUT/out.json"
{ echo "# One-page reviewer reproduction guide"; echo; echo "Steps $(jq -r .steps "$OUT/out.json"); within budget $(jq -r .within_step_budget "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "reviewer-reproduction-guide worker: within_step_budget=$(jq -r .within_step_budget "$OUT/out.json")"
