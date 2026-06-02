#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/ecosystem-certification-gate.json}"; OUT="${2:-results/generated/ecosystem-certification}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.ecosystem-certification-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "conformance" "make ecosystem-certification-gate"; do grep -F "$phrase" docs/ecosystem-certification.md README.md > /dev/null; done
bash scripts/ecosystem-certification.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.ecosystem-certification/v1" and .certified==true and .bad_certified==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.ecosystem-certification-gate-results/v1",certified:$r[0].certified,bad_denied:($r[0].bad_certified|not),verified:true}' > "$OUT/gate-summary.json"
echo "ecosystem-certification gate passed: conforming extension certified, non-conforming extension denied"
