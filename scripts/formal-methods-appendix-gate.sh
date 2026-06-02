#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/formal-methods-appendix-gate.json}"; OUT="${2:-results/generated/formal-methods-appendix}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.formal-methods-appendix-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "machine-checked" "make formal-methods-appendix-gate"; do grep -F "$phrase" docs/formal-methods-appendix.md README.md > /dev/null; done
bash scripts/formal-methods-appendix.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.formal-methods-appendix/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.formal-methods-appendix-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "formal-methods-appendix gate passed: every proof CI machine-checked, unchecked proof rejected"
