#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/polyglot-orm-frontend-gate.json}"; OUT="${2:-results/generated/polyglot-orm-frontend}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.polyglot-orm-frontend-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "framework" "make polyglot-orm-frontend-gate"; do grep -F "$phrase" docs/polyglot-orm-frontend.md README.md > /dev/null; done
bash scripts/polyglot-orm-frontend.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.polyglot-orm-frontend/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.polyglot-orm-frontend-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "polyglot-orm-frontend gate passed: every framework lowers to shared core, extractor-less framework rejected"
