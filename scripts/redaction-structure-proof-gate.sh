#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/redaction-structure-proof-gate.json}"; OUT="${2:-results/generated/redaction-structure-proof}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.redaction-structure-proof-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "redaction preserves" "make redaction-structure-proof-gate"; do grep -F "$phrase" docs/redaction-structure-proof.md README.md > /dev/null; done
bash scripts/redaction-structure-proof.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.redaction-structure-proof/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.redaction-structure-proof-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "redaction-structure-proof gate passed: every item scored with evidence on real self-data, unsupported item rejected"
