#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/proof-minimization-engine-gate.json}"; OUT="${2:-results/generated/proof-minimization-engine}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.proof-minimization-engine-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "proof minimization" "make proof-minimization-engine-gate"; do grep -F "$phrase" docs/proof-minimization-engine.md README.md > /dev/null; done
bash scripts/proof-minimization-engine.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.proof-minimization-engine/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.proof-minimization-engine-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "proof-minimization-engine gate passed: every item scored with evidence on real self-data, unsupported item rejected"
