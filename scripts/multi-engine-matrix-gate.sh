#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/multi-engine-matrix-gate.json}"; OUT="${2:-results/generated/multi-engine-matrix}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.multi-engine-matrix-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "per-engine" "make multi-engine-matrix-gate"; do grep -F "$phrase" docs/multi-engine-matrix.md README.md > /dev/null; done
bash scripts/multi-engine-matrix.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.multi-engine-matrix/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.multi-engine-matrix-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "multi-engine-matrix gate passed: every engine has per-engine semantics with cases, undefined engine rejected"
