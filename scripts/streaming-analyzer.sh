#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/streaming-analyzer-gate.json}"; OUT="${2:-results/generated/streaming-analyzer}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.streaming-analyzer-gate/v1" and (.severities|length) >= 1' "$SPEC" > /dev/null
jq '
  .window as $W | .severities as $S
  | (reduce $S[] as $x ({count:0, max:0, buf:[], peak:0};
       .count += 1
       | .max = (if $x > .max then $x else .max end)
       | .buf = ((.buf + [$x]) | if length > $W then .[1:] else . end)
       | .peak = (if (.buf|length) > .peak then (.buf|length) else .peak end)
     )) as $stream
  | {
      version: "patchline.streaming-analyzer/v1",
      window: $W,
      total: ($S|length),
      stream_count: $stream.count,
      stream_max: $stream.max,
      peak_in_memory: $stream.peak,
      bounded: ($stream.peak <= $W),
      batch_count: ($S|length),
      batch_max: ($S|max),
      aggregates_match: ($stream.count == ($S|length) and $stream.max == ($S|max)),
      buffer_all_would_retain: ($S|length)
    }
' "$SPEC" > "$OUT/stream.json"
{ echo "# Streaming bounded-memory analyzer"; echo; echo "Peak in memory: $(jq -r '.peak_in_memory' "$OUT/stream.json")/$(jq -r '.total' "$OUT/stream.json") (bound $(jq -r '.window' "$OUT/stream.json"))"; echo "Aggregates match batch: $(jq -r '.aggregates_match' "$OUT/stream.json")"; } > "$OUT/stream.md"
cp "$OUT/stream.md" "$OUT/README.md"
echo "streaming-analyzer worker: peak=$(jq -r '.peak_in_memory' "$OUT/stream.json") bounded=$(jq -r '.bounded' "$OUT/stream.json") match=$(jq -r '.aggregates_match' "$OUT/stream.json")"
