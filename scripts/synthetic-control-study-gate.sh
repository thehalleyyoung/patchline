#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/synthetic-control-study-gate.json}"; OUT="${2:-results/generated/synthetic-control-study}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.synthetic-control-study-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "synthetic control" "make synthetic-control-study-gate"; do grep -F "$phrase" docs/synthetic-control-study.md README.md > /dev/null; done
bash scripts/synthetic-control-study.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.synthetic-control-study/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.synthetic-control-study-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "synthetic-control-study gate passed: every item scored with evidence on real self-data, unsupported item rejected"
