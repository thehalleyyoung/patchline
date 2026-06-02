#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/theorem-prover-backend-gate.json}"; OUT="${2:-results/generated/theorem-prover-backend}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.theorem-prover-backend-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "proof" "make theorem-prover-backend-gate"; do grep -F "$phrase" docs/theorem-prover-backend.md README.md > /dev/null; done
bash scripts/theorem-prover-backend.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.theorem-prover-backend/v1" and .all_proved==true and .proved_rate==1 and .unprovable_proved==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.theorem-prover-backend-gate-results/v1",proved_rate:$r[0].proved_rate,all_proved:$r[0].all_proved,unprovable_reported:($r[0].unprovable_proved|not),verified:true}' > "$OUT/gate-summary.json"
echo "theorem-prover-backend gate passed: all sound obligations proved with proof objects, unprovable reported unproved"
