#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/external-auditor-repro-gate.json}"; OUT="${2:-results/generated/external-auditor-repro}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.external-auditor-repro-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "attestation" "make external-auditor-repro-gate"; do grep -F "$phrase" docs/external-auditor-repro.md README.md > /dev/null; done
bash scripts/external-auditor-repro.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.external-auditor-repro/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.external-auditor-repro-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "external-auditor-repro gate passed: every headline result reproduced and signed, unsigned reproduction rejected"
