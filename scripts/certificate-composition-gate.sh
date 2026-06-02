#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/certificate-composition-gate.json}"; OUT="${2:-results/generated/certificate-composition}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.certificate-composition-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "compose" "make certificate-composition-gate"; do grep -F "$phrase" docs/certificate-composition.md README.md > /dev/null; done
bash scripts/certificate-composition.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.certificate-composition/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.certificate-composition-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "certificate-composition gate passed: all certificate pairs compose consistently, contradictory pair rejected"
