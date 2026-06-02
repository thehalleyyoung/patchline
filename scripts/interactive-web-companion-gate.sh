#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/interactive-web-companion-gate.json}"; OUT="${2:-results/generated/interactive-web-companion}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.interactive-web-companion-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "live gates" "make interactive-web-companion-gate"; do grep -F "$phrase" docs/interactive-web-companion.md README.md > /dev/null; done
bash scripts/interactive-web-companion.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.interactive-web-companion/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.interactive-web-companion-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "interactive-web-companion gate passed: every item scored with evidence on real self-data, unsupported item rejected"
