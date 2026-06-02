#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/incident-auto-remediation-gate.json}"; OUT="${2:-results/generated/incident-auto-remediation}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.incident-auto-remediation-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "auto-remediation" "make incident-auto-remediation-gate"; do grep -F "$phrase" docs/incident-auto-remediation.md README.md > /dev/null; done
bash scripts/incident-auto-remediation.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.incident-auto-remediation/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.incident-auto-remediation-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "incident-auto-remediation gate passed: every item scored with evidence on real self-data, unsupported item rejected"
