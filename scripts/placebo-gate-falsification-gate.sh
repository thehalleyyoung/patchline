#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/placebo-gate-falsification-gate.json}"; OUT="${2:-results/generated/placebo-gate-falsification}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.placebo-gate-falsification-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "placebo gate" "make placebo-gate-falsification-gate"; do grep -F "$phrase" docs/placebo-gate-falsification.md README.md > /dev/null; done
bash scripts/placebo-gate-falsification.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.placebo-gate-falsification/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.placebo-gate-falsification-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "placebo-gate-falsification gate passed: every item scored with evidence on real self-data, unsupported item rejected"
