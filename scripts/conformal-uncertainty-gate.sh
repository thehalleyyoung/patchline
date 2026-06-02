#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/conformal-uncertainty-gate.json}"; OUT="${2:-results/generated/conformal-uncertainty}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.conformal-uncertainty-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "coverage" "make conformal-uncertainty-gate"; do grep -F "$phrase" docs/conformal-uncertainty.md README.md > /dev/null; done
bash scripts/conformal-uncertainty.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.conformal-uncertainty/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.conformal-uncertainty-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "conformal-uncertainty gate passed: every set meets coverage, undercovering set rejected"
