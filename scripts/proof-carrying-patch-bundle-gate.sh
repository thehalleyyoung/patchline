#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/proof-carrying-patch-bundle-gate.json}"; OUT="${2:-results/generated/proof-carrying-patch-bundle}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.proof-carrying-patch-bundle-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "proof-carrying patch" "make proof-carrying-patch-bundle-gate"; do grep -F "$phrase" docs/proof-carrying-patch-bundle.md README.md > /dev/null; done
bash scripts/proof-carrying-patch-bundle.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.proof-carrying-patch-bundle/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.proof-carrying-patch-bundle-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "proof-carrying-patch-bundle gate passed: every item scored with evidence on real self-data, unsupported item rejected"
