#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/learned-risk-model-gate.json}"; OUT="${2:-results/generated/learned-risk-model}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.learned-risk-model-gate/v1"' "$SPEC" > /dev/null
jq '

  def r4: (.*10000|round)/10000;
  .holdout as $H
  | ([ $H[] | select((.pred>=0.5) == (.label==1)) ]|length) as $correct
  | (($H|map((.pred-.label)*(.pred-.label))|add)/($H|length)) as $brier
  | {version:"patchline.learned-risk-model/v1",
     n:($H|length),
     accuracy:(($correct/($H|length))|r4),
     brier:($brier|r4),
     beats_baseline:((($correct/($H|length))) > .majority_accuracy),
     held_out:(.evaluated_split != .train_split)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Learned risk model"; echo; echo "Accuracy $(jq -r .accuracy "$OUT/out.json"); Brier $(jq -r .brier "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "learned-risk-model worker: accuracy=$(jq -r .accuracy "$OUT/out.json") held_out=$(jq -r .held_out "$OUT/out.json")"
