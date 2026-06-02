#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/usage-metering-gate.json}"; OUT="${2:-results/generated/usage-metering}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.usage-metering-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "invoice" "make usage-metering-gate"; do grep -F "$phrase" docs/usage-metering.md README.md > /dev/null; done
bash scripts/usage-metering.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.usage-metering/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.usage-metering-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "usage-metering gate passed: every invoice event-reproducible, non-reproducible invoice rejected"
