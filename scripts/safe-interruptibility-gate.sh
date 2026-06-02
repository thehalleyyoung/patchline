#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/safe-interruptibility-gate.json}"; OUT="${2:-results/generated/safe-interruptibility}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.safe-interruptibility-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "safe interruptibility" "make safe-interruptibility-gate"; do grep -F "$phrase" docs/safe-interruptibility.md README.md > /dev/null; done
bash scripts/safe-interruptibility.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.safe-interruptibility/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.safe-interruptibility-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "safe-interruptibility gate passed: every item scored with evidence on real self-data, unsupported item rejected"
