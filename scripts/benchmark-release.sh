#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/benchmark-release-gate.json}"; OUT="${2:-results/generated/benchmark-release}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.benchmark-release-gate/v1" and (.train|length) >= 1 and (.test|length) >= 1' "$SPEC" > /dev/null
jq '
  # Deterministic checksum: codepoint sum over the canonical sorted split string.
  def checksum($train; $test): (((($train|sort) + ["|"] + ($test|sort)) | join(",")) | explode | add);
  def valid_submission($s; $ver):
    ($s.benchmark_version == $ver) and (($s | has("predictions"))) and (($s.predictions | type) == "array")
    and ($s.predictions | all(.[]; (has("id") and has("label"))));
  .benchmark_version as $ver | .train as $tr | .test as $te
  | (checksum($tr; $te)) as $c1 | (checksum($tr; $te)) as $c2
  | {
      version: "patchline.benchmark-release/v1",
      benchmark_version: $ver,
      checksum: $c1,
      checksum_stable: ($c1 == $c2),
      disjoint: ((($tr|sort) - ($te|sort)) == ($tr|sort) and (($tr - $te) | length) == ($tr|length) and (([ $tr[] | select([ . == $te[] ] | any) ]) | length) == 0),
      complete: ((($tr + $te) | sort | length) == (($tr + $te) | unique | length)),
      good_submission_valid: valid_submission(.good_submission; $ver),
      bad_submission_valid: valid_submission(.bad_submission; $ver)
    }
' "$SPEC" > "$OUT/bench.json"
{ echo "# Versioned benchmark release"; echo; echo "Checksum stable: $(jq -r '.checksum_stable' "$OUT/bench.json"); disjoint: $(jq -r '.disjoint' "$OUT/bench.json")"; echo "Good submission valid: $(jq -r '.good_submission_valid' "$OUT/bench.json"); bad: $(jq -r '.bad_submission_valid' "$OUT/bench.json")"; } > "$OUT/bench.md"
cp "$OUT/bench.md" "$OUT/README.md"
echo "benchmark-release worker: stable=$(jq -r '.checksum_stable' "$OUT/bench.json") good=$(jq -r '.good_submission_valid' "$OUT/bench.json") bad=$(jq -r '.bad_submission_valid' "$OUT/bench.json")"
