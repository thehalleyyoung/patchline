#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/fp-adjudication-gate.json}"
OUT="${2:-results/generated/fp-adjudication-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.fp-adjudication-gate/v1" and (.minimum_raters | numbers)' "$SPEC" > /dev/null

for phrase in "False-positive adjudication" "blinded" "Cohen's kappa" "majority-adjudicated false-positive rate" "make fp-adjudication-gate"; do
  grep -F "$phrase" docs/fp-adjudication.md README.md > /dev/null
done

bash scripts/fp-adjudication.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in fp-adjudication.json fp-adjudication.md blinded-examples.jsonl rater-labels.jsonl README.md; do
  test -s "$OUT/$output"
done

min_examples="$(jq '.minimum_examples' "$SPEC")"
min_kappa="$(jq '.minimum_mean_kappa' "$SPEC")"

jq -e --argjson min_examples "$min_examples" --argjson min_kappa "$min_kappa" '
  .version == "patchline.fp-adjudication/v1" and
  .examples >= $min_examples and
  .summary.raters == 3 and
  .summary.verified == true and
  .mean_kappa >= $min_kappa and
  (.pairwise_kappa | length) == 3 and
  (.full_agreement_rate >= 0 and .full_agreement_rate <= 1) and
  (.majority_true_positive_rate + .majority_false_positive_rate | (. * 1000 | round)) == 1000 and
  (.rater_true_positive_rates | to_entries | all(.[]; .value > 0 and .value < 1))
' "$OUT/fp-adjudication.json" > /dev/null

# Blinded examples must NOT leak severity/score; raters must use only neutral fields.
jq -e -s 'all(.[]; has("severity") | not) and all(.[]; has("score") | not) and all(.[]; has("has_danger_evidence") and (.operation_kind | type == "string"))' "$OUT/blinded-examples.jsonl" > /dev/null

# Every rater row must carry three boolean votes and a derived majority verdict.
test "$(wc -l < "$OUT/rater-labels.jsonl" | tr -d ' ')" -ge "$min_examples"
jq -e -s 'all(.[]; (.rater_evidence|type=="boolean") and (.rater_control_gap|type=="boolean") and (.rater_operation|type=="boolean") and (.majority_tp|type=="boolean"))' "$OUT/rater-labels.jsonl" > /dev/null

jq -n \
  --slurpfile report "$OUT/fp-adjudication.json" \
  '{
    version: "patchline.fp-adjudication-gate-results/v1",
    examples: $report[0].examples,
    mean_kappa: $report[0].mean_kappa,
    full_agreement_rate: $report[0].full_agreement_rate,
    majority_false_positive_rate: $report[0].majority_false_positive_rate,
    verified: true
  }' > "$OUT/gate-summary.json"

echo "fp adjudication gate passed: examples $(jq '.examples' "$OUT/gate-summary.json"), mean kappa $(jq '.mean_kappa' "$OUT/gate-summary.json"), 3-way agreement $(jq '.full_agreement_rate' "$OUT/gate-summary.json"), majority FP rate $(jq '.majority_false_positive_rate' "$OUT/gate-summary.json")"
