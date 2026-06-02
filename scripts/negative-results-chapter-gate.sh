#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/negative-results-chapter-gate.json}"; OUT="${2:-results/generated/negative-results-chapter}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.negative-results-chapter-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "negative-results" "make negative-results-chapter-gate"; do grep -F "$phrase" docs/negative-results-chapter.md README.md > /dev/null; done
bash scripts/negative-results-chapter.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.negative-results-chapter/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.negative-results-chapter-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "negative-results-chapter gate passed: every item scored with evidence on real self-data, unsupported item rejected"
