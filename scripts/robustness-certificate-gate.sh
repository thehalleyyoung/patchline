#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/robustness-certificate-gate.json}"; OUT="${2:-results/generated/robustness-certificate}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.robustness-certificate-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "bounded perturbations" "make robustness-certificate-gate"; do grep -F "$phrase" docs/robustness-certificate.md README.md > /dev/null; done
bash scripts/robustness-certificate.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.robustness-certificate/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.robustness-certificate-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "robustness-certificate gate passed: every item scored with evidence on real self-data, unsupported item rejected"
