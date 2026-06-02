#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/orm-upstream-contribution-gate.json}"; OUT="${2:-results/generated/orm-upstream-contribution}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.orm-upstream-contribution-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "upstream" "make orm-upstream-contribution-gate"; do grep -F "$phrase" docs/orm-upstream-contribution.md README.md > /dev/null; done
bash scripts/orm-upstream-contribution.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.orm-upstream-contribution/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.orm-upstream-contribution-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "orm-upstream-contribution gate passed: every item scored with evidence on real self-data, unsupported item rejected"
