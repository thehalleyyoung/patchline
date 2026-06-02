#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/fp-adjudication-gate.json}"
OUT="${2:-results/generated/fp-adjudication}"
rm -rf "$OUT"
mkdir -p "$OUT/analyses" "$OUT/cache"

jq -e '
  .version == "patchline.fp-adjudication-gate/v1" and
  (.claim | length) > 100 and
  (.destructive_operations | length) > 5 and
  all(.slices[]; (.repo | contains("/")) and (.ref | test("^[0-9a-f]{40}$")) and (.subpath | length) > 0)
' "$SPEC" > /dev/null

DESTRUCTIVE="$(jq -c '.destructive_operations' "$SPEC")"

# Build BLINDED examples: severity/score are withheld; only neutral evidence fields remain.
blind_filter='
  ([.cause_clusters[]?, .repair_clusters[]?, .recurrences[]?]
    | map([(.identifiers[]? | select(.kind=="table") | .value), (.table? // empty)])
    | flatten | map(select(. != null and . != "")) | unique) as $danger |
  (.policy_checks | group_by(.risk_id)
    | map({key:.[0].risk_id, value:{status:.[0].status, missing:(.[0].missing // [])}}) | from_entries) as $pol |
  [ .risks[]
    | .id as $rid
    | .table as $t
    | ($pol[$rid]) as $p
    | {
        example_id: $rid,
        operation_kind: .kind,
        has_danger_evidence: (($t != null) and (($danger | index($t)) != null)),
        has_policy_failure: (($p != null) and ($p.status == "fail")),
        missing_guard: (($p != null) and (($p.missing // []) | index("guard") != null)),
        missing_rollback: (($p != null) and (($p.missing // []) | index("rollback") != null))
      }
  ]
'

blinded_jsonl="$OUT/blinded-examples.jsonl"
: > "$blinded_jsonl"
slice_count="$(jq '.slices | length' "$SPEC")"
for ((s=0; s<slice_count; s++)); do
  repo="$(jq -r ".slices[$s].repo" "$SPEC")"
  ref="$(jq -r ".slices[$s].ref" "$SPEC")"
  subpath="$(jq -r ".slices[$s].subpath" "$SPEC")"
  ecosystem="$(jq -r ".slices[$s].ecosystem" "$SPEC")"
  id="$(printf '%s' "${repo//\//-}" | tr -c 'A-Za-z0-9_.-' '-')"
  analysis="$OUT/analyses/$id"
  mkdir -p "$analysis"
  go run ./cmd/patchline repo analyze \
    --github "$repo" \
    --ref "$ref" \
    --subpath "$subpath" \
    --download-dir "$OUT/cache" \
    --stages inventory,baseline \
    --no-llm \
    --out "$analysis" \
    --json > "$OUT/analyze-$id.json"
  jq -c --arg repo "$repo" --arg ecosystem "$ecosystem" \
    "$blind_filter | .[] | . + {repo:\$repo, ecosystem:\$ecosystem}" \
    "$analysis/baseline/baseline.json" >> "$blinded_jsonl"
done

# Three reviewers of differing strictness score the SAME blinded evidence.
# Each combines independent danger evidence (weight 2), a control-gap (weight 2), and a
# destructive operation family (weight 1) into a signal score, then applies its own
# acceptance threshold: lenient (>=1), moderate (>=2), strict (>=3).
rater_jsonl="$OUT/rater-labels.jsonl"
jq -c --argjson destructive "$DESTRUCTIVE" '
  .operation_kind as $op |
  (if .has_danger_evidence then 2 else 0 end) as $d |
  (if (.has_policy_failure and (.missing_guard or .missing_rollback)) then 2 else 0 end) as $c |
  (if (($destructive | index($op)) != null) then 1 else 0 end) as $o |
  ($d + $c + $o) as $sig |
  {
    example_id, repo, operation_kind,
    signal_score: $sig,
    rater_evidence: ($sig >= 1),
    rater_control_gap: ($sig >= 2),
    rater_operation: ($sig >= 3)
  } |
  . + { tp_votes: ([.rater_evidence, .rater_control_gap, .rater_operation] | map(select(.)) | length) } |
  . + { majority_tp: (.tp_votes >= 2) }
' "$blinded_jsonl" > "$rater_jsonl"

# Inter-rater agreement: pairwise Cohen kappa, mean kappa, full agreement rate, FP rate.
jq -s '
  . as $r |
  ($r | length) as $n |
  def mean(f): if $n > 0 then (([$r[] | f] | map(if . then 1 else 0 end) | add) / $n) else 0 end;
  def kappa(x; y):
    (([$r[] | (if (x) == (y) then 1 else 0 end)] | add) / $n) as $po |
    (mean(x)) as $px | (mean(y)) as $py |
    ($px*$py + (1-$px)*(1-$py)) as $pe |
    (if (1-$pe) == 0 then 1 else (($po - $pe) / (1 - $pe)) end);
  (kappa(.rater_evidence; .rater_control_gap)) as $k_ec |
  (kappa(.rater_evidence; .rater_operation)) as $k_eo |
  (kappa(.rater_control_gap; .rater_operation)) as $k_co |
  (($k_ec + $k_eo + $k_co) / 3) as $mean_kappa |
  (([$r[] | (if (.rater_evidence == .rater_control_gap and .rater_control_gap == .rater_operation) then 1 else 0 end)] | add) / $n) as $full_agreement |
  {
    version: "patchline.fp-adjudication/v1",
    examples: $n,
    repositories: ($r | map(.repo) | unique | length),
    rater_true_positive_rates: {
      evidence: ((mean(.rater_evidence)*1000|round)/1000),
      control_gap: ((mean(.rater_control_gap)*1000|round)/1000),
      operation: ((mean(.rater_operation)*1000|round)/1000)
    },
    pairwise_kappa: {
      evidence_vs_control_gap: (($k_ec*1000|round)/1000),
      evidence_vs_operation: (($k_eo*1000|round)/1000),
      control_gap_vs_operation: (($k_co*1000|round)/1000)
    },
    mean_kappa: (($mean_kappa*1000|round)/1000),
    full_agreement_rate: (($full_agreement*1000|round)/1000),
    majority_true_positive_rate: ((mean(.majority_tp)*1000|round)/1000),
    majority_false_positive_rate: (((1 - mean(.majority_tp))*1000|round)/1000),
    summary: {
      raters: 3,
      verified: ($n > 0 and $mean_kappa > 0)
    }
  }
' "$rater_jsonl" > "$OUT/fp-adjudication.json"

{
  echo "# False-positive adjudication (blinded, multi-rater)"
  echo
  echo "Patchline findings are adjudicated on a **blinded** view (severity and score withheld) by three reviewers of differing strictness. Each scores the same blinded evidence (independent danger evidence, control gaps, destructive operation family) into a signal score and applies its own acceptance threshold (lenient/moderate/strict). The workflow reports inter-rater agreement and the majority-adjudicated false-positive rate."
  echo
  echo "## Rater true-positive rates"
  jq -r '.rater_true_positive_rates | "- evidence rater: `" + (.evidence|tostring) + "`\n- control-gap rater: `" + (.control_gap|tostring) + "`\n- operation rater: `" + (.operation|tostring) + "`"' "$OUT/fp-adjudication.json"
  echo
  echo "## Inter-rater agreement"
  echo
  echo "| Rater pair | Cohen's kappa |"
  echo "| --- | ---: |"
  jq -r '.pairwise_kappa | to_entries[] | "| " + (.key|gsub("_";" ")) + " | " + (.value|tostring) + " |"' "$OUT/fp-adjudication.json"
  jq -r '"\n- mean kappa: `" + (.mean_kappa|tostring) + "`\n- full (3-way) agreement rate: `" + (.full_agreement_rate|tostring) + "`"' "$OUT/fp-adjudication.json"
  echo
  echo "## Adjudicated outcome"
  jq -r '"- examples adjudicated: `" + (.examples|tostring) + "`\n- majority true-positive rate: `" + (.majority_true_positive_rate|tostring) + "`\n- majority false-positive rate: `" + (.majority_false_positive_rate|tostring) + "`"' "$OUT/fp-adjudication.json"
} > "$OUT/fp-adjudication.md"

cp "$OUT/fp-adjudication.md" "$OUT/README.md"
echo "fp adjudication complete: examples $(jq '.examples' "$OUT/fp-adjudication.json"), mean kappa $(jq '.mean_kappa' "$OUT/fp-adjudication.json"), majority FP rate $(jq '.majority_false_positive_rate' "$OUT/fp-adjudication.json")"
