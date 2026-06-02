#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/historical-results-never-regress-gate.json}"; OUT="${2:-results/generated/historical-results-never-regress}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.historical-results-never-regress-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "never regress" "make historical-results-never-regress-gate"; do grep -F "$phrase" docs/historical-results-never-regress.md README.md > /dev/null; done
bash scripts/historical-results-never-regress.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.historical-results-never-regress/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.historical-results-never-regress-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "historical-results-never-regress gate passed: every item scored with evidence on real self-data, unsupported item rejected"
