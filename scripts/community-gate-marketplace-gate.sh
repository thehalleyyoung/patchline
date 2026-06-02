#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/community-gate-marketplace-gate.json}"; OUT="${2:-results/generated/community-gate-marketplace}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.community-gate-marketplace-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "signing" "make community-gate-marketplace-gate"; do grep -F "$phrase" docs/community-gate-marketplace.md README.md > /dev/null; done
bash scripts/community-gate-marketplace.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.community-gate-marketplace/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.community-gate-marketplace-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "community-gate-marketplace gate passed: every item scored with evidence on real self-data, unsupported item rejected"
