#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/streaming-differential-analyzer-gate.json}"; OUT="${2:-results/generated/streaming-differential-analyzer}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.streaming-differential-analyzer-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "hazard delta" "make streaming-differential-analyzer-gate"; do grep -F "$phrase" docs/streaming-differential-analyzer.md README.md > /dev/null; done
bash scripts/streaming-differential-analyzer.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.streaming-differential-analyzer/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.streaming-differential-analyzer-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "streaming-differential-analyzer gate passed: every item scored with evidence on real self-data, unsupported item rejected"
