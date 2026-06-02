#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/threats-to-validity-gate.json}"; OUT="${2:-results/generated/threats-to-validity}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.threats-to-validity-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "threats to validity" "make threats-to-validity-gate"; do grep -F "$phrase" docs/threats-to-validity.md README.md > /dev/null; done
bash scripts/threats-to-validity.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.threats-to-validity/v1" and .all_backed==true and .unbacked_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.threats-to-validity-gate-results/v1",backed:$r[0].backed,all_backed:$r[0].all_backed,unbacked_rejected:($r[0].unbacked_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "threats-to-validity gate passed: every threat backed by an experiment, unbacked threat rejected"
