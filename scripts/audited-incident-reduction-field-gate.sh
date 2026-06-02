#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/audited-incident-reduction-field-gate.json}"; OUT="${2:-results/generated/audited-incident-reduction-field}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.audited-incident-reduction-field-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "independently-audited" "make audited-incident-reduction-field-gate"; do grep -F "$phrase" docs/audited-incident-reduction-field.md README.md > /dev/null; done
bash scripts/audited-incident-reduction-field.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.audited-incident-reduction-field/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.audited-incident-reduction-field-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "audited-incident-reduction-field gate passed: every item scored with evidence on real self-data, unsupported item rejected"
