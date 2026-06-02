#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/ci-marketplace-actions-gate.json}"; OUT="${2:-results/generated/ci-marketplace-actions}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.ci-marketplace-actions-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "CI marketplace" "make ci-marketplace-actions-gate"; do grep -F "$phrase" docs/ci-marketplace-actions.md README.md > /dev/null; done
bash scripts/ci-marketplace-actions.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.ci-marketplace-actions/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.ci-marketplace-actions-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "ci-marketplace-actions gate passed: every item scored with evidence on real self-data, unsupported item rejected"
