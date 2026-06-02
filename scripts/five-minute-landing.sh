#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/five-minute-landing-gate.json}"; OUT="${2:-results/generated/five-minute-landing}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.five-minute-landing-gate/v1"' "$SPEC" > /dev/null
jq '

  def r4: (.*10000|round)/10000;
  .friction_flow as $F
  | {version:"patchline.five-minute-landing/v1",
     completion_rate:((.completions/.starts)|r4),
     within_time:(.median_minutes <= .max_minutes),
     clears_threshold:((.completions/.starts) >= .min_completion),
     friction_clears:(($F.completions/$F.starts) >= .min_completion)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Five-minute landing flow"; echo; echo "Completion $(jq -r .completion_rate "$OUT/out.json"); within time $(jq -r .within_time "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "five-minute-landing worker: completion_rate=$(jq -r .completion_rate "$OUT/out.json")"
