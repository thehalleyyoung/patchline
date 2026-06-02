#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/provenance-linked-every-number-gate.json}"; OUT="${2:-results/generated/provenance-linked-every-number}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.provenance-linked-every-number-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "every-number provenance" "make provenance-linked-every-number-gate"; do grep -F "$phrase" docs/provenance-linked-every-number.md README.md > /dev/null; done
bash scripts/provenance-linked-every-number.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.provenance-linked-every-number/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.provenance-linked-every-number-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "provenance-linked-every-number gate passed: every item scored with evidence on real self-data, unsupported item rejected"
