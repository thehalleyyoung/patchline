#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/explainable-ranking-gate.json}"
OUT="${2:-results/generated/explainable-ranking}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.explainable-ranking-gate/v1" and (.items|length) >= 2 and (.weights|type=="object")' "$SPEC" > /dev/null

# Score = weighted sum of signals; attribute to per-signal contributions; re-rank with
# the dominant signal removed.
jq '
  .weights as $w
  | .dominant_signal as $dom
  | def score($item; $weights):
      ($item.signals | to_entries | map(.value * ($weights[.key] // 0)) | add);
  def contribs($item; $weights):
      ($item.signals | to_entries | map({signal: .key, contribution: (.value * ($weights[.key] // 0))}));
  ($w | to_entries | map(select(.key != $dom)) | from_entries) as $w_reduced
  | {
      version: "patchline.explainable-ranking/v1",
      ranking: ([ .items[] | { id: .id, score: score(.; $w), contributions: contribs(.; $w) } ] | sort_by(-.score)),
      ranking_without_dominant: ([ .items[] | { id: .id, score: score(.; $w_reduced) } ] | sort_by(-.score))
    }
  | . + {
      top: .ranking[0].id,
      top_without_dominant: .ranking_without_dominant[0].id,
      contributions_sum_ok: (.ranking | all(.[]; (.contributions | map(.contribution) | add) as $s | (($s - .score) | if . < 0 then -. else . end) < 0.0000001)),
      dominant_is_load_bearing: (.ranking[0].id != .ranking_without_dominant[0].id)
    }
' "$SPEC" > "$OUT/explainable-ranking.json"

{
  echo "# Explainable ranking"
  echo
  echo "Top item: $(jq -r .top "$OUT/explainable-ranking.json")"
  echo
  echo "| Item | Score | Top contribution |"
  echo "|---|---|---|"
  jq -r '.ranking[] | "| \(.id) | \(.score) | \(.contributions | max_by(.contribution) | .signal) |"' "$OUT/explainable-ranking.json"
  echo
  echo "Top after removing dominant signal: $(jq -r .top_without_dominant "$OUT/explainable-ranking.json")"
} > "$OUT/explainable-ranking.md"
cp "$OUT/explainable-ranking.md" "$OUT/README.md"

echo "explainable-ranking worker: top=$(jq -r .top "$OUT/explainable-ranking.json") top_without_dominant=$(jq -r .top_without_dominant "$OUT/explainable-ranking.json")"
