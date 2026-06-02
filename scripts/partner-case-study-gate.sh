#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/partner-case-study-gate.json}"; OUT="${2:-results/generated/partner-case-study}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.partner-case-study-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "signed" "make partner-case-study-gate"; do grep -F "$phrase" docs/partner-case-study.md README.md > /dev/null; done
bash scripts/partner-case-study.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.partner-case-study/v1" and .all_valid==true and .unsigned_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.partner-case-study-gate-results/v1",valid:$r[0].valid,all_valid:$r[0].all_valid,unsigned_rejected:($r[0].unsigned_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "partner-case-study gate passed: every case study signed and reproducible, unsigned bundle rejected"
