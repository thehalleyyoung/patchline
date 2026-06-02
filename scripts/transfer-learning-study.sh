#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/transfer-learning-study-gate.json}"; OUT="${2:-results/generated/transfer-learning-study}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.transfer-learning-study-gate/v1"' "$SPEC" > /dev/null
jq '

  .train_ecosystems as $T | .test_ecosystem as $te
  | {version:"patchline.transfer-learning-study/v1",
     disjoint:(($T|index($te))==null),
     zero_shot_accuracy:.zero_shot_accuracy,
     clears_threshold:(.zero_shot_accuracy >= .threshold),
     leaked_disjoint:((.leaked_train|index($te))==null)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Transfer-learning study"; echo; echo "Zero-shot acc $(jq -r .zero_shot_accuracy "$OUT/out.json"); disjoint $(jq -r .disjoint "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "transfer-learning-study worker: disjoint=$(jq -r .disjoint "$OUT/out.json") clears=$(jq -r .clears_threshold "$OUT/out.json")"
