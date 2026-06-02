#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/theorem-discovery-loop-gate.json}"; OUT="${2:-results/generated/theorem-discovery-loop}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.theorem-discovery-loop-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "theorem-discovery" "make theorem-discovery-loop-gate"; do grep -F "$phrase" docs/theorem-discovery-loop.md README.md > /dev/null; done
bash scripts/theorem-discovery-loop.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.theorem-discovery-loop/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.theorem-discovery-loop-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "theorem-discovery-loop gate passed: every item scored with evidence on real self-data, unsupported item rejected"
