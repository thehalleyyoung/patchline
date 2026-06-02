#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/streaming-state-transfer-repair-gate.json}"; OUT="${2:-results/generated/streaming-state-transfer-repair}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.streaming-state-transfer-repair-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "streaming state transfer" "make streaming-state-transfer-repair-gate"; do grep -F "$phrase" docs/streaming-state-transfer-repair.md README.md > /dev/null; done
bash scripts/streaming-state-transfer-repair.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.streaming-state-transfer-repair/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.streaming-state-transfer-repair-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "streaming-state-transfer-repair gate passed: every item scored with evidence on real self-data, unsupported item rejected"
