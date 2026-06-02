#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/macro-hygiene-dsl-extension-gate.json}"; OUT="${2:-results/generated/macro-hygiene-dsl-extension}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.macro-hygiene-dsl-extension-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "macro-hygiene" "make macro-hygiene-dsl-extension-gate"; do grep -F "$phrase" docs/macro-hygiene-dsl-extension.md README.md > /dev/null; done
bash scripts/macro-hygiene-dsl-extension.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.macro-hygiene-dsl-extension/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.macro-hygiene-dsl-extension-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "macro-hygiene-dsl-extension gate passed: every item scored with evidence on real self-data, unsupported item rejected"
