#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/proof-carrying-verdict-gate.json}"; OUT="${2:-results/generated/proof-carrying-verdict}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.proof-carrying-verdict-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "proof witness" "make proof-carrying-verdict-gate"; do grep -F "$phrase" docs/proof-carrying-verdict.md README.md > /dev/null; done
bash scripts/proof-carrying-verdict.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.proof-carrying-verdict/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.proof-carrying-verdict-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "proof-carrying-verdict gate passed: every item scored with evidence on real self-data, unsupported item rejected"
