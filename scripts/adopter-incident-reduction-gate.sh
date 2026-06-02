#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/adopter-incident-reduction-gate.json}"; OUT="${2:-results/generated/adopter-incident-reduction}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.adopter-incident-reduction-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "incident rate" "make adopter-incident-reduction-gate"; do grep -F "$phrase" docs/adopter-incident-reduction.md README.md > /dev/null; done
bash scripts/adopter-incident-reduction.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.adopter-incident-reduction/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.adopter-incident-reduction-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "adopter-incident-reduction gate passed: every adopter's incident rate dropped, rate-increase adopter rejected"
