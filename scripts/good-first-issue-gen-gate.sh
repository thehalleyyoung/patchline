#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/good-first-issue-gen-gate.json}"; OUT="${2:-results/generated/good-first-issue-gen}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.good-first-issue-gen-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "good first issue" "make good-first-issue-gen-gate"; do grep -F "$phrase" docs/good-first-issue-gen.md README.md > /dev/null; done
bash scripts/good-first-issue-gen.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.good-first-issue-gen/v1" and .all_backed==true and .fabricated_backed==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.good-first-issue-gen-gate-results/v1",actionable:$r[0].actionable,all_backed:$r[0].all_backed,fabricated_rejected:($r[0].fabricated_backed|not),verified:true}' > "$OUT/gate-summary.json"
echo "good-first-issue-gen gate passed: every issue references a real gap and is scoped, fabricated issue rejected"
