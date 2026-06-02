#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/mechanistic-feature-study-gate.json}"; OUT="${2:-results/generated/mechanistic-feature-study}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.mechanistic-feature-study-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "mechanistic" "make mechanistic-feature-study-gate"; do grep -F "$phrase" docs/mechanistic-feature-study.md README.md > /dev/null; done
bash scripts/mechanistic-feature-study.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.mechanistic-feature-study/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.mechanistic-feature-study-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "mechanistic-feature-study gate passed: every item scored with evidence on real self-data, unsupported item rejected"
