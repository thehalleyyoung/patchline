#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/framework-holdout-gate.json}"; OUT="${2:-results/generated/framework-holdout}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.framework-holdout-gate/v1"' "$SPEC" > /dev/null
jq '
  .holdout_framework as $h
  | {
      version: "patchline.framework-holdout/v1",
      train_frameworks: (.train_frameworks|sort),
      holdout_framework: $h,
      no_leakage: ([ $h == .train_frameworks[] ] | any | not),
      threshold_selected_on: (.train_frameworks|sort),
      evaluated_on: $h,
      leaked_no_leakage: ([ $h == .leaked_train_frameworks[] ] | any | not)
    }
' "$SPEC" > "$OUT/holdout.json"
{ echo "# Framework holdout generalization"; echo; echo "Train: $(jq -rc '.train_frameworks' "$OUT/holdout.json"); holdout: $(jq -r '.holdout_framework' "$OUT/holdout.json")"; echo "No leakage: $(jq -r '.no_leakage' "$OUT/holdout.json"); leaked config clean: $(jq -r '.leaked_no_leakage' "$OUT/holdout.json")"; } > "$OUT/holdout.md"
cp "$OUT/holdout.md" "$OUT/README.md"
echo "framework-holdout worker: no_leakage=$(jq -r '.no_leakage' "$OUT/holdout.json") leaked_clean=$(jq -r '.leaked_no_leakage' "$OUT/holdout.json")"
