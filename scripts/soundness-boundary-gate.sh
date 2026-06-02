#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/soundness-boundary-gate.json}"; OUT="${2:-results/generated/soundness-boundary}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.soundness-boundary-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "soundness boundary" "make soundness-boundary-gate"; do grep -F "$phrase" docs/soundness-boundary.md README.md > /dev/null; done
bash scripts/soundness-boundary.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.soundness-boundary/v1" and .all_leveled==true and .all_guaranteed_backed==true and .ungated_backed==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.soundness-boundary-gate-results/v1",all_guaranteed_backed:$r[0].all_guaranteed_backed,ungated_rejected:($r[0].ungated_backed|not),verified:true}' > "$OUT/gate-summary.json"
echo "soundness-boundary gate passed: boundary total, every guarantee backed by a gate, ungated guarantee rejected"
