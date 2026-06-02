#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/ablation-study-gate.json}"
OUT="${2:-results/generated/ablation-study}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.ablation-study-gate/v1" and
  (.claim | length) > 100 and
  ([.axes[] | select(.kind == "score-factor")] | length) >= .minimum_score_factor_axes and
  all(.axes[] | select(.kind == "score-factor"); (.factors | length) > 0)
' "$SPEC" > /dev/null

repo="$(jq -r '.real_repo.repo' "$SPEC")"
ref="$(jq -r '.real_repo.ref' "$SPEC")"
subpath="$(jq -r '.real_repo.subpath' "$SPEC")"

go run ./cmd/patchline repo analyze \
  --github "$repo" --ref "$ref" --subpath "$subpath" \
  --download-dir "$OUT/cache" --stages inventory,baseline --no-llm \
  --out "$OUT/analysis" --json > "$OUT/analyze.log"

BASE="$OUT/analysis/baseline/baseline.json"

# Ablation engine: for each score-factor axis, remove the listed factors' weights, re-score,
# and measure affected risks, total weight removed, mean score drop, and top-K displacement.
jq \
  --argjson top_k "$(jq '.top_k' "$SPEC")" \
  --argjson high_thr "$(jq '.high_score_threshold' "$SPEC")" \
  --argjson budgets "$(jq '.budgets' "$SPEC")" \
  --arg rt_pat "$(jq -r '.runtime_evidence_pattern' "$SPEC")" \
  --argjson axes "$(jq '.axes' "$SPEC")" \
  --arg repo "$repo" '
  .risks as $risks |
  (.repair_proof_summaries // []) as $proofs |
  (.proof_hole_minimizations // []) as $holes |
  ($risks | length) as $n |

  # Full ranking: ids sorted by score desc, take top_k.
  ($risks | map({id, score}) | sort_by(-.score)) as $full_rank |
  ($full_rank[0:$top_k] | map(.id)) as $full_topk |
  ($risks | map(select(.score >= $high_thr) | .id)) as $high_cohort |
  ($risks | map(.score) | add) as $total_score |

  # Score-factor axis ablations.
  ( [ $axes[] | select(.kind == "score-factor") | . as $ax |
      ( $risks | map(
          . as $r |
          ( [ $r.factors[]? | select(.name as $fn | $ax.factors | index($fn)) | .weight ] | add // 0 ) as $rm |
          { id: $r.id, removed: $rm, ablated_score: ($r.score - $rm) }
        ) ) as $abl |
      ( $abl | map(select(.removed > 0)) ) as $affected |
      ( $abl | sort_by(-.ablated_score)[0:$top_k] | map(.id) ) as $new_topk |
      {
        axis: $ax.id,
        kind: "score-factor",
        affected_risks: ($affected | length),
        total_weight_removed: ($affected | map(.removed) | add // 0),
        mean_score_drop: (if ($affected | length) > 0 then (($affected | map(.removed) | add) / ($affected | length) * 100 | round) / 100 else 0 end),
        topk_displaced: ([ $full_topk[] | select(. as $id | $new_topk | index($id) | not) ] | length),
        score_share_removed: (if $total_score > 0 then (($affected | map(.removed) | add // 0) / $total_score * 1000 | round) / 1000 else 0 end)
      }
    ] ) as $factor_results |

  # Runtime-traces axis: proof obligations that only runtime/production evidence can close.
  ( ($proofs | map(select(.status != "refuted")) | length) ) as $unresolved |
  ( ($holes | map(select((.missing_evidence // "" | test($rt_pat)) or (.hole // "" | test($rt_pat)))) | length) ) as $runtime_holes |
  {
    axis: "runtime-traces",
    kind: "proof-obligation",
    total_proof_summaries: ($proofs | length),
    unresolved_without_runtime: $unresolved,
    unresolved_fraction: (if ($proofs | length) > 0 then ($unresolved / ($proofs | length) * 1000 | round) / 1000 else 0 end),
    runtime_dependent_holes: $runtime_holes
  } as $runtime_result |

  # Risk-budgets axis: capture of total score and of the high cohort under top-K budgets.
  ( [ $budgets[] | . as $k |
      ( $full_rank[0:$k] | map(.id) ) as $sel |
      {
        budget: $k,
        score_capture: (if $total_score > 0 then (($full_rank[0:$k] | map(.score) | add // 0) / $total_score * 1000 | round) / 1000 else 0 end),
        high_cohort_recall: (if ($high_cohort | length) > 0 then ([ $high_cohort[] | select(. as $id | $sel | index($id)) ] | length) / ($high_cohort | length) else 0 end)
      }
    ] ) as $budget_curve |
  {
    axis: "risk-budgets",
    kind: "budget-selection",
    high_cohort_size: ($high_cohort | length),
    curve: $budget_curve,
    monotonic: ([ $budget_curve[].score_capture ] | . as $c | [range(0; ($c|length)-1) | $c[.] <= $c[.+1]] | all)
  } as $budget_result |

  {
    version: "patchline.ablation-study/v1",
    repo: $repo,
    total_risks: $n,
    score_factor_axes: $factor_results,
    runtime_traces: $runtime_result,
    risk_budgets: $budget_result,
    ranking_by_impact: ($factor_results | sort_by(-.score_share_removed) | map(.axis))
  }
' "$BASE" > "$OUT/ablation-study.json"

{
  echo "# Ablation study (real downloaded baseline)"
  echo
  jq -r '"Repository: `" + .repo + "`, total risks: `" + (.total_risks|tostring) + "`."' "$OUT/ablation-study.json"
  echo
  echo "## Score-factor ablations"
  echo
  echo "| Axis | Affected risks | Weight removed | Score share | Top-K displaced |"
  echo "| --- | ---: | ---: | ---: | ---: |"
  jq -r '.score_factor_axes[] | "| " + .axis + " | " + (.affected_risks|tostring) + " | " + (.total_weight_removed|tostring) + " | " + (.score_share_removed|tostring) + " | " + (.topk_displaced|tostring) + " |"' "$OUT/ablation-study.json"
  echo
  jq -r '"Ranking by ablation impact (score share removed): " + (.ranking_by_impact | join(" > "))' "$OUT/ablation-study.json"
  echo
  echo "## Runtime-traces ablation (proof obligations)"
  jq -r '.runtime_traces | "- proof summaries: `" + (.total_proof_summaries|tostring) + "`\n- unresolved without runtime evidence: `" + (.unresolved_without_runtime|tostring) + "` (fraction `" + (.unresolved_fraction|tostring) + "`)\n- runtime-dependent proof holes: `" + (.runtime_dependent_holes|tostring) + "`"' "$OUT/ablation-study.json"
  echo
  echo "## Risk-budgets ablation (top-K selection)"
  echo
  echo "| Budget | Score capture | High-cohort recall |"
  echo "| ---: | ---: | ---: |"
  jq -r '.risk_budgets.curve[] | "| " + (.budget|tostring) + " | " + (.score_capture|tostring) + " | " + ((.high_cohort_recall*1000|round)/1000|tostring) + " |"' "$OUT/ablation-study.json"
} > "$OUT/ablation-study.md"

cp "$OUT/ablation-study.md" "$OUT/README.md"
echo "ablation study complete: risks $(jq '.total_risks' "$OUT/ablation-study.json"), top impact $(jq -r '.ranking_by_impact[0]' "$OUT/ablation-study.json"), runtime unresolved frac $(jq '.runtime_traces.unresolved_fraction' "$OUT/ablation-study.json")"
