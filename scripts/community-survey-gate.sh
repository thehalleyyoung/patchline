#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/community-survey-gate.json}"; OUT="${2:-results/generated/community-survey}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.community-survey-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "survey" "make community-survey-gate"; do grep -F "$phrase" docs/community-survey.md README.md > /dev/null; done
bash scripts/community-survey.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.community-survey/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.community-survey-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "community-survey gate passed: every cycle published and roadmap-driving, unpublished cycle rejected"
