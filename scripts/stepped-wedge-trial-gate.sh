#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/stepped-wedge-trial-gate.json}"; OUT="${2:-results/generated/stepped-wedge-trial}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.stepped-wedge-trial-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "stepped-wedge trial" "make stepped-wedge-trial-gate"; do grep -F "$phrase" docs/stepped-wedge-trial.md README.md > /dev/null; done
bash scripts/stepped-wedge-trial.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.stepped-wedge-trial/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.stepped-wedge-trial-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "stepped-wedge-trial gate passed: every item scored with evidence on real self-data, unsupported item rejected"
