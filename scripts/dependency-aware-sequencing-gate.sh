#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/dependency-aware-sequencing-gate.json}"; OUT="${2:-results/generated/dependency-aware-sequencing}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.dependency-aware-sequencing-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "dependency-aware sequencing" "make dependency-aware-sequencing-gate"; do grep -F "$phrase" docs/dependency-aware-sequencing.md README.md > /dev/null; done
bash scripts/dependency-aware-sequencing.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.dependency-aware-sequencing/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.dependency-aware-sequencing-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "dependency-aware-sequencing gate passed: every item scored with evidence on real self-data, unsupported item rejected"
