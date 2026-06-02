#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/inline-review-surface-gate.json}"; OUT="${2:-results/generated/inline-review-surface}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.inline-review-surface-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "inline" "make inline-review-surface-gate"; do grep -F "$phrase" docs/inline-review-surface.md README.md > /dev/null; done
bash scripts/inline-review-surface.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.inline-review-surface/v1" and .all_anchored==true and .broken_anchored==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.inline-review-surface-gate-results/v1",all_anchored:$r[0].all_anchored,broken_rejected:($r[0].broken_anchored|not),verified:true}' > "$OUT/gate-summary.json"
echo "inline-review-surface gate passed: every finding anchored with reproduce command, broken anchor rejected"
