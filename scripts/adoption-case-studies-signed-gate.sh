#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/adoption-case-studies-signed-gate.json}"; OUT="${2:-results/generated/adoption-case-studies-signed}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.adoption-case-studies-signed-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "signed" "make adoption-case-studies-signed-gate"; do grep -F "$phrase" docs/adoption-case-studies-signed.md README.md > /dev/null; done
bash scripts/adoption-case-studies-signed.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.adoption-case-studies-signed/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.adoption-case-studies-signed-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "adoption-case-studies-signed gate passed: every case study signed, unsigned bundle rejected"
