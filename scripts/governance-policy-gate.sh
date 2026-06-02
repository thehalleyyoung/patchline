#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/governance-policy-gate.json}"; OUT="${2:-results/generated/governance-policy}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.governance-policy-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "deprecation" "make governance-policy-gate"; do grep -F "$phrase" docs/governance-policy.md README.md > /dev/null; done
bash scripts/governance-policy.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.governance-policy/v1" and .semver==true and .breaking_compliant==true and .rushed_compliant==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.governance-policy-gate-results/v1",semver:$r[0].semver,breaking_compliant:$r[0].breaking_compliant,rushed_rejected:($r[0].rushed_compliant|not),verified:true}' > "$OUT/gate-summary.json"
echo "governance-policy gate passed: semver and deprecation policy satisfied, rushed breaking change rejected"
