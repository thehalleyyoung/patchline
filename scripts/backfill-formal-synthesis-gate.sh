#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/backfill-formal-synthesis-gate.json}"; OUT="${2:-results/generated/backfill-formal-synthesis}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.backfill-formal-synthesis-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "provably-correct" "make backfill-formal-synthesis-gate"; do grep -F "$phrase" docs/backfill-formal-synthesis.md README.md > /dev/null; done
bash scripts/backfill-formal-synthesis.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.backfill-formal-synthesis/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.backfill-formal-synthesis-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "backfill-formal-synthesis gate passed: every synthesized backfill establishes its invariant, no-op rejected"
