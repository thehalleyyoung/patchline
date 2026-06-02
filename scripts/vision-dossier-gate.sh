#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/vision-dossier-gate.json}"; OUT="${2:-results/generated/vision-dossier}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.vision-dossier-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "novelty" "make vision-dossier-gate"; do grep -F "$phrase" docs/vision-dossier.md README.md > /dev/null; done
bash scripts/vision-dossier.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.vision-dossier/v1" and .all_covered==true and .incomplete_covered==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.vision-dossier-gate-results/v1",all_covered:$r[0].all_covered,incomplete_rejected:($r[0].incomplete_covered|not),verified:true}' > "$OUT/gate-summary.json"
echo "vision-dossier gate passed: all four pillars gate-backed, incomplete dossier rejected"
