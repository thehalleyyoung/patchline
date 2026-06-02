#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/fleet-slo-enforcer-gate.json}"; OUT="${2:-results/generated/fleet-slo-enforcer}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.fleet-slo-enforcer-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "fleet SLO" "make fleet-slo-enforcer-gate"; do grep -F "$phrase" docs/fleet-slo-enforcer.md README.md > /dev/null; done
bash scripts/fleet-slo-enforcer.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.fleet-slo-enforcer/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.fleet-slo-enforcer-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "fleet-slo-enforcer gate passed: every item scored with evidence on real self-data, unsupported item rejected"
