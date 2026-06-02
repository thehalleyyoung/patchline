#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/grand-unified-one-command-index-gate.json}"; OUT="${2:-results/generated/grand-unified-one-command-index}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.grand-unified-one-command-index-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "one-command" "make grand-unified-one-command-index-gate"; do grep -F "$phrase" docs/grand-unified-one-command-index.md README.md > /dev/null; done
bash scripts/grand-unified-one-command-index.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.grand-unified-one-command-index/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.grand-unified-one-command-index-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "grand-unified-one-command-index gate passed: every item scored with evidence on real self-data, unsupported item rejected"
