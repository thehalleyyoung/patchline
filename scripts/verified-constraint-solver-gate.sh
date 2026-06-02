#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/verified-constraint-solver-gate.json}"; OUT="${2:-results/generated/verified-constraint-solver}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.verified-constraint-solver-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "verified constraint solver" "make verified-constraint-solver-gate"; do grep -F "$phrase" docs/verified-constraint-solver.md README.md > /dev/null; done
bash scripts/verified-constraint-solver.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.verified-constraint-solver/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.verified-constraint-solver-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "verified-constraint-solver gate passed: every item scored with evidence on real self-data, unsupported item rejected"
