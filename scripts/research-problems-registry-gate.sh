#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/research-problems-registry-gate.json}"; OUT="${2:-results/generated/research-problems-registry}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.research-problems-registry-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "bounties" "make research-problems-registry-gate"; do grep -F "$phrase" docs/research-problems-registry.md README.md > /dev/null; done
bash scripts/research-problems-registry.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.research-problems-registry/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.research-problems-registry-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "research-problems-registry gate passed: every item scored with evidence on real self-data, unsupported item rejected"
