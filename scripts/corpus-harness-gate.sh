#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/corpus-harness-gate.json}"; OUT="${2:-results/generated/corpus-harness}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.corpus-harness-gate/v1" and (.claim|length) > 200 and (.repos|length) >= 10' "$SPEC" > /dev/null
for phrase in "shard" "make corpus-harness-gate"; do grep -F "$phrase" docs/corpus-harness.md README.md > /dev/null; done
bash scripts/corpus-harness.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '
  .version == "patchline.corpus-harness/v1" and
  .deterministic == true and .all_in_range == true and
  .resume_excludes_completed == true and
  (.remaining_count == (.total - 3)) and (.remaining_count < .total)
' "$OUT/harness.json" > /dev/null
jq -n --slurpfile r "$OUT/harness.json" '{version:"patchline.corpus-harness-gate-results/v1", deterministic:$r[0].deterministic, shard_sizes:$r[0].shard_sizes, remaining:$r[0].remaining_count, verified:true}' > "$OUT/gate-summary.json"
echo "corpus-harness gate passed: deterministic sharding, in-range shards, resumable sweep"
