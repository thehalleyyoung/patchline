#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/office-hours-triage-sla-gate.json}"; OUT="${2:-results/generated/office-hours-triage-sla}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.office-hours-triage-sla-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "response-time SLA" "make office-hours-triage-sla-gate"; do grep -F "$phrase" docs/office-hours-triage-sla.md README.md > /dev/null; done
bash scripts/office-hours-triage-sla.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.office-hours-triage-sla/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.office-hours-triage-sla-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "office-hours-triage-sla gate passed: every item scored with evidence on real self-data, unsupported item rejected"
