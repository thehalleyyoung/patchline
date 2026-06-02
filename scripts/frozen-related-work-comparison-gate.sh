#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/frozen-related-work-comparison-gate.json}"; OUT="${2:-results/generated/frozen-related-work-comparison}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.frozen-related-work-comparison-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "frozen benchmark" "make frozen-related-work-comparison-gate"; do grep -F "$phrase" docs/frozen-related-work-comparison.md README.md > /dev/null; done
bash scripts/frozen-related-work-comparison.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.frozen-related-work-comparison/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.frozen-related-work-comparison-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "frozen-related-work-comparison gate passed: every item scored with evidence on real self-data, unsupported item rejected"
