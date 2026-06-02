#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/quickstart-sixty-seconds-gate.json}"; OUT="${2:-results/generated/quickstart-sixty-seconds}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.quickstart-sixty-seconds-gate/v1"' "$SPEC" > /dev/null
jq '

  .budget_seconds as $b | .phases as $P
  | ([ $P[].seconds ] | add) as $tot
  | {version:"patchline.quickstart-sixty-seconds/v1",
     budget:$b, total_seconds:$tot, phases:($P|length),
     within_budget:($tot <= $b),
     slow_within_budget:(.slow_run_total <= $b)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Sixty-second quickstart"; echo; echo "Total $(jq -r .total_seconds "$OUT/out.json")s / budget $(jq -r .budget "$OUT/out.json")s"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "quickstart-sixty-seconds worker: within_budget=$(jq -r .within_budget "$OUT/out.json")"
