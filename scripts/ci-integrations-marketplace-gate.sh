#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/ci-integrations-marketplace-gate.json}"; OUT="${2:-results/generated/ci-integrations-marketplace}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.ci-integrations-marketplace-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "verified" "make ci-integrations-marketplace-gate"; do grep -F "$phrase" docs/ci-integrations-marketplace.md README.md > /dev/null; done
bash scripts/ci-integrations-marketplace.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.ci-integrations-marketplace/v1" and .all_verified==true and .verified_rate==1 and .unverified_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.ci-integrations-marketplace-gate-results/v1",verified_rate:$r[0].verified_rate,all_verified:$r[0].all_verified,unverified_rejected:($r[0].unverified_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "ci-integrations-marketplace gate passed: every CI integration verified with a recipe, unverified listing rejected"
