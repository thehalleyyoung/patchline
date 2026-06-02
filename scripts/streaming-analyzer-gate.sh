#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/streaming-analyzer-gate.json}"; OUT="${2:-results/generated/streaming-analyzer}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.streaming-analyzer-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "memory" "make streaming-analyzer-gate"; do grep -F "$phrase" docs/streaming-analyzer.md README.md > /dev/null; done
bash scripts/streaming-analyzer.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '
  .version == "patchline.streaming-analyzer/v1" and
  .bounded == true and
  (.peak_in_memory <= .window) and
  .aggregates_match == true and
  (.buffer_all_would_retain > .peak_in_memory)
' "$OUT/stream.json" > /dev/null
jq -n --slurpfile r "$OUT/stream.json" '{version:"patchline.streaming-analyzer-gate-results/v1", peak:$r[0].peak_in_memory, bound:$r[0].window, aggregates_match:$r[0].aggregates_match, verified:true}' > "$OUT/gate-summary.json"
echo "streaming-analyzer gate passed: bounded peak memory, aggregate equals batch"
