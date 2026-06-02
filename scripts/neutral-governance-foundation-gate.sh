#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/neutral-governance-foundation-gate.json}"; OUT="${2:-results/generated/neutral-governance-foundation}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.neutral-governance-foundation-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "neutral foundation" "make neutral-governance-foundation-gate"; do grep -F "$phrase" docs/neutral-governance-foundation.md README.md > /dev/null; done
bash scripts/neutral-governance-foundation.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.neutral-governance-foundation/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.neutral-governance-foundation-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "neutral-governance-foundation gate passed: every item scored with evidence on real self-data, unsupported item rejected"
