#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/difference-in-differences-gate.json}"; OUT="${2:-results/generated/difference-in-differences}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.difference-in-differences-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "parallel-trends" "make difference-in-differences-gate"; do grep -F "$phrase" docs/difference-in-differences.md README.md > /dev/null; done
bash scripts/difference-in-differences.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.difference-in-differences/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.difference-in-differences-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "difference-in-differences gate passed: every item scored with evidence on real self-data, unsupported item rejected"
