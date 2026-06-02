#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/abstraction-selection-policy-gate.json}"; OUT="${2:-results/generated/abstraction-selection-policy}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.abstraction-selection-policy-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "never weaken soundness" "make abstraction-selection-policy-gate"; do grep -F "$phrase" docs/abstraction-selection-policy.md README.md > /dev/null; done
bash scripts/abstraction-selection-policy.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.abstraction-selection-policy/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.abstraction-selection-policy-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "abstraction-selection-policy gate passed: every item scored with evidence on real self-data, unsupported item rejected"
