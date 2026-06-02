#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/end-to-end-provenance-gate.json}"; OUT="${2:-results/generated/end-to-end-provenance}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.end-to-end-provenance-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "provenance" "make end-to-end-provenance-gate"; do grep -F "$phrase" docs/end-to-end-provenance.md README.md > /dev/null; done
bash scripts/end-to-end-provenance.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.end-to-end-provenance/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.end-to-end-provenance-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "end-to-end-provenance gate passed: every paper number provenance-traced, untraceable number rejected"
