#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/signed-provenance-chain-gate.json}"; OUT="${2:-results/generated/signed-provenance-chain}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.signed-provenance-chain-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "provenance" "make signed-provenance-chain-gate"; do grep -F "$phrase" docs/signed-provenance-chain.md README.md > /dev/null; done
bash scripts/signed-provenance-chain.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.signed-provenance-chain/v1" and .intact==true and .all_signed==true and .terminal=="verdict" and .broken_intact==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.signed-provenance-chain-gate-results/v1",intact:$r[0].intact,all_signed:$r[0].all_signed,broken_rejected:($r[0].broken_intact|not),verified:true}' > "$OUT/gate-summary.json"
echo "signed-provenance-chain gate passed: full chain intact and signed, broken digest link rejected"
