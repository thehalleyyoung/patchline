#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/human-override-audit-trail-gate.json}"; OUT="${2:-results/generated/human-override-audit-trail}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.human-override-audit-trail-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "reversible" "make human-override-audit-trail-gate"; do grep -F "$phrase" docs/human-override-audit-trail.md README.md > /dev/null; done
bash scripts/human-override-audit-trail.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.human-override-audit-trail/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.human-override-audit-trail-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "human-override-audit-trail gate passed: every item scored with evidence on real self-data, unsupported item rejected"
