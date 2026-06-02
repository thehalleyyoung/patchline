#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/fix-suggestion-engine-gate.json}"; OUT="${2:-results/generated/fix-suggestion-engine}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.fix-suggestion-engine-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "safe migration variant" "make fix-suggestion-engine-gate"; do grep -F "$phrase" docs/fix-suggestion-engine.md README.md > /dev/null; done
bash scripts/fix-suggestion-engine.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.fix-suggestion-engine/v1" and .all_remediated==true and .coverage==1 and .bogus_clears==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.fix-suggestion-engine-gate-results/v1",coverage:$r[0].coverage,all_remediated:$r[0].all_remediated,bogus_rejected:($r[0].bogus_clears|not),verified:true}' > "$OUT/gate-summary.json"
echo "fix-suggestion-engine gate passed: every hazard gets a verdict-clearing fix, bogus fix rejected"
