#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/incident-simulation-env-gate.json}"; OUT="${2:-results/generated/incident-simulation-env}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.incident-simulation-env-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "counterfactual" "make incident-simulation-env-gate"; do grep -F "$phrase" docs/incident-simulation-env.md README.md > /dev/null; done
bash scripts/incident-simulation-env.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.incident-simulation-env/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.incident-simulation-env-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "incident-simulation-env gate passed: every simulated timeline valid, malformed timeline rejected"
