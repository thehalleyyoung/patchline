#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/best-paper-readiness-gate.json}"; OUT="${2:-results/generated/best-paper-readiness}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.best-paper-readiness-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "rubric" "make best-paper-readiness-gate"; do grep -F "$phrase" docs/best-paper-readiness.md README.md > /dev/null; done
bash scripts/best-paper-readiness.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.best-paper-readiness/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.best-paper-readiness-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "best-paper-readiness gate passed: every rubric criterion scored with evidence, unsupported criterion rejected"
