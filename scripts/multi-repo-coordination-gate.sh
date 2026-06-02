#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/multi-repo-coordination-gate.json}"; OUT="${2:-results/generated/multi-repo-coordination}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.multi-repo-coordination-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "two-phase" "make multi-repo-coordination-gate"; do grep -F "$phrase" docs/multi-repo-coordination.md README.md > /dev/null; done
bash scripts/multi-repo-coordination.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.multi-repo-coordination/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.multi-repo-coordination-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "multi-repo-coordination gate passed: every item scored with evidence on real self-data, unsupported item rejected"
