#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/ten-million-migration-mining-gate.json}"; OUT="${2:-results/generated/ten-million-migration-mining}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.ten-million-migration-mining-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "content-addressed" "make ten-million-migration-mining-gate"; do grep -F "$phrase" docs/ten-million-migration-mining.md README.md > /dev/null; done
bash scripts/ten-million-migration-mining.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.ten-million-migration-mining/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.ten-million-migration-mining-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "ten-million-migration-mining gate passed: every item scored with evidence on real self-data, unsupported item rejected"
