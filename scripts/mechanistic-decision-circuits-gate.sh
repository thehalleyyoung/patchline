#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/mechanistic-decision-circuits-gate.json}"; OUT="${2:-results/generated/mechanistic-decision-circuits}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.mechanistic-decision-circuits-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "decision circuits" "make mechanistic-decision-circuits-gate"; do grep -F "$phrase" docs/mechanistic-decision-circuits.md README.md > /dev/null; done
bash scripts/mechanistic-decision-circuits.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.mechanistic-decision-circuits/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.mechanistic-decision-circuits-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "mechanistic-decision-circuits gate passed: every item scored with evidence on real self-data, unsupported item rejected"
