#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/showcase-gallery-gate.json}"; OUT="${2:-results/generated/showcase-gallery}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.showcase-gallery-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "reproducible evidence" "make showcase-gallery-gate"; do grep -F "$phrase" docs/showcase-gallery.md README.md > /dev/null; done
bash scripts/showcase-gallery.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.showcase-gallery/v1" and .all_backed==true and .unbacked_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.showcase-gallery-gate-results/v1",backed:$r[0].backed,all_backed:$r[0].all_backed,unbacked_rejected:($r[0].unbacked_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "showcase-gallery gate passed: every showcase entry has reproducible evidence, unbacked entry rejected"
