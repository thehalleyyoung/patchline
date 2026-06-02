#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/bit-identical-rebuild-gate.json}"; OUT="${2:-results/generated/bit-identical-rebuild}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.bit-identical-rebuild-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "bit-identical" "make bit-identical-rebuild-gate"; do grep -F "$phrase" docs/bit-identical-rebuild.md README.md > /dev/null; done
bash scripts/bit-identical-rebuild.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.bit-identical-rebuild/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.bit-identical-rebuild-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "bit-identical-rebuild gate passed: every release bit-identical on rebuild, non-deterministic build rejected"
