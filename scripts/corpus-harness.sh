#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/corpus-harness-gate.json}"; OUT="${2:-results/generated/corpus-harness}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.corpus-harness-gate/v1" and (.repos|length) >= 1' "$SPEC" > /dev/null
jq '
  # Deterministic shard: sum of codepoints of the repo id modulo shard count.
  def shard($id; $n): ([ $id | explode[] ] | add) % $n;
  .shards as $n | .repos as $R | .completed as $done
  | (reduce $R[] as $r ({}; .[$r] = shard($r; $n))) as $assign1
  | (reduce $R[] as $r ({}; .[$r] = shard($r; $n))) as $assign2
  | {
      version: "patchline.corpus-harness/v1",
      shards: $n,
      total: ($R|length),
      assignment: $assign1,
      deterministic: ($assign1 == $assign2),
      all_in_range: ([ $assign1[] | (. >= 0 and . < $n) ] | all),
      shard_sizes: (reduce ($assign1|to_entries[]) as $e ({}; .[($e.value|tostring)] = ((.[($e.value|tostring)] // 0) + 1))),
      remaining: ([ $R[] | select([ . == $done[] ] | any | not) ]),
      remaining_count: ([ $R[] | select([ . == $done[] ] | any | not) ] | length),
      resume_excludes_completed: ([ $R[] | select([ . == $done[] ] | any | not) ] | (map(select([ . == $done[] ] | any)) | length) == 0)
    }
' "$SPEC" > "$OUT/harness.json"
{ echo "# Corpus sweep harness"; echo; echo "Deterministic: $(jq -r '.deterministic' "$OUT/harness.json"); shards: $(jq -rc '.shard_sizes' "$OUT/harness.json")"; echo "Remaining after resume: $(jq -r '.remaining_count' "$OUT/harness.json")/$(jq -r '.total' "$OUT/harness.json")"; } > "$OUT/harness.md"
cp "$OUT/harness.md" "$OUT/README.md"
echo "corpus-harness worker: deterministic=$(jq -r '.deterministic' "$OUT/harness.json") remaining=$(jq -r '.remaining_count' "$OUT/harness.json")"
