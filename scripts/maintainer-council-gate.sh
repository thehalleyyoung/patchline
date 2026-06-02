#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/maintainer-council-gate.json}"; OUT="${2:-results/generated/maintainer-council}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.maintainer-council-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "decision record" "make maintainer-council-gate"; do grep -F "$phrase" docs/maintainer-council.md README.md > /dev/null; done
bash scripts/maintainer-council.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.maintainer-council/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.maintainer-council-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "maintainer-council gate passed: every governance element documented, undocumented element rejected"
