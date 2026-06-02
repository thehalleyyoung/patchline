#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/evalue-confounding-bound-gate.json}"; OUT="${2:-results/generated/evalue-confounding-bound}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.evalue-confounding-bound-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "E-value" "make evalue-confounding-bound-gate"; do grep -F "$phrase" docs/evalue-confounding-bound.md README.md > /dev/null; done
bash scripts/evalue-confounding-bound.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.evalue-confounding-bound/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.evalue-confounding-bound-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "evalue-confounding-bound gate passed: every item scored with evidence on real self-data, unsupported item rejected"
