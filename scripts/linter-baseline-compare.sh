#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/linter-baseline-compare-gate.json}"; OUT="${2:-results/generated/linter-baseline-compare}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.linter-baseline-compare-gate/v1"' "$SPEC" > /dev/null
jq '
  def r4: (.*10000|round)/10000;
  .cases as $cases | .gold_hazards as $gold
  | {
      version: "patchline.linter-baseline-compare/v1",
      matched: (.tools | to_entries | all(.[]; (.value.cases | sort) == ($cases|sort))),
      recall: (.tools | map_values( . as $t | (([ $gold[] | select([ . == $t.flagged[] ] | any) ] | length) / ($gold|length)) | r4 )),
      mismatched_rejected: ((.mismatched_tool.cases | sort) != ($cases|sort))
    }
  | .patchline_best = (.recall.patchline >= .recall.linter_a and .recall.patchline >= .recall.linter_b
                       and .recall.patchline > .recall.linter_a and .recall.patchline > .recall.linter_b)
' "$SPEC" > "$OUT/compare.json"
{ echo "# Baseline linter comparison"; echo; echo "Recall: $(jq -rc '.recall' "$OUT/compare.json")"; echo "Patchline best: $(jq -r '.patchline_best' "$OUT/compare.json"); matched: $(jq -r '.matched' "$OUT/compare.json")"; } > "$OUT/compare.md"
cp "$OUT/compare.md" "$OUT/README.md"
echo "linter-baseline-compare worker: recall=$(jq -rc '.recall' "$OUT/compare.json") best=$(jq -r '.patchline_best' "$OUT/compare.json")"
