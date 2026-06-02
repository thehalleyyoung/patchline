#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/backfill-synthesis-gate.json}"; OUT="${2:-results/generated/backfill-synthesis}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.backfill-synthesis-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "invariant" "make backfill-synthesis-gate"; do grep -F "$phrase" docs/backfill-synthesis.md README.md > /dev/null; done
bash scripts/backfill-synthesis.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.backfill-synthesis/v1" and .establishes_invariant==true and .noop_establishes==false and .steps>=3' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.backfill-synthesis-gate-results/v1",establishes_invariant:$r[0].establishes_invariant,noop_fails:($r[0].noop_establishes|not),verified:true}' > "$OUT/gate-summary.json"
echo "backfill-synthesis gate passed: synthesized backfill establishes the invariant, no-op fails"
