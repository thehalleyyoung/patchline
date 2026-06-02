#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/instrumental-variable-estimate-gate.json}"; OUT="${2:-results/generated/instrumental-variable-estimate}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.instrumental-variable-estimate-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "instrumental-variable" "make instrumental-variable-estimate-gate"; do grep -F "$phrase" docs/instrumental-variable-estimate.md README.md > /dev/null; done
bash scripts/instrumental-variable-estimate.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.instrumental-variable-estimate/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.instrumental-variable-estimate-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "instrumental-variable-estimate gate passed: every item scored with evidence on real self-data, unsupported item rejected"
