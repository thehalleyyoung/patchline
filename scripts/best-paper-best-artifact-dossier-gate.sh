#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/best-paper-best-artifact-dossier-gate.json}"; OUT="${2:-results/generated/best-paper-best-artifact-dossier}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.best-paper-best-artifact-dossier-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "award rubrics" "make best-paper-best-artifact-dossier-gate"; do grep -F "$phrase" docs/best-paper-best-artifact-dossier.md README.md > /dev/null; done
bash scripts/best-paper-best-artifact-dossier.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.best-paper-best-artifact-dossier/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.best-paper-best-artifact-dossier-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "best-paper-best-artifact-dossier gate passed: every item scored with evidence on real self-data, unsupported item rejected"
