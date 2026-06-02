#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/third-party-security-audit-gate.json}"; OUT="${2:-results/generated/third-party-security-audit}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.third-party-security-audit-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "remediated" "make third-party-security-audit-gate"; do grep -F "$phrase" docs/third-party-security-audit.md README.md > /dev/null; done
bash scripts/third-party-security-audit.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.third-party-security-audit/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.third-party-security-audit-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "third-party-security-audit gate passed: every finding remediated and re-verified, open finding rejected"
