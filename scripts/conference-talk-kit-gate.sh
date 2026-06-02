#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/conference-talk-kit-gate.json}"; OUT="${2:-results/generated/conference-talk-kit}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.conference-talk-kit-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "live demo" "make conference-talk-kit-gate"; do grep -F "$phrase" docs/conference-talk-kit.md README.md > /dev/null; done
bash scripts/conference-talk-kit.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.conference-talk-kit/v1" and .all_backed==true and .unbacked_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.conference-talk-kit-gate-results/v1",backed:$r[0].backed,all_backed:$r[0].all_backed,unbacked_rejected:($r[0].unbacked_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "conference-talk-kit gate passed: every live demo gate-backed, unbacked segment rejected"
