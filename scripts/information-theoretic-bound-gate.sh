#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/information-theoretic-bound-gate.json}"; OUT="${2:-results/generated/information-theoretic-bound}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.information-theoretic-bound-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "information-theoretic" "make information-theoretic-bound-gate"; do grep -F "$phrase" docs/information-theoretic-bound.md README.md > /dev/null; done
bash scripts/information-theoretic-bound.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.information-theoretic-bound/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.information-theoretic-bound-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "information-theoretic-bound gate passed: every item scored with evidence on real self-data, unsupported item rejected"
