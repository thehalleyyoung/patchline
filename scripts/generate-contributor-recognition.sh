#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/contributor-recognition-gate.json}"
OUT="${2:-results/generated/contributor-recognition}"
rm -rf "$OUT"
mkdir -p "$OUT/proofs" "$OUT/cards"

jq -e '
  .version == "patchline.contributor-recognition-gate/v1" and
  (.claim | length) > 140 and
  (.contributors | length) >= .minimum_contributors and
  (.required_categories | length) >= .minimum_categories and
  all(.proof_gates[]; (.id | test("^[a-z0-9-]+$")) and (.script | startswith("scripts/")) and (.spec | startswith("examples/")) and (.summary | length) > 0) and
  all(.contributors[]; (.id | test("^[a-z0-9-]+$")) and (.display | length) > 0 and (.category | length) > 0 and (.proof_gate | length) > 0 and (.badge | length) > 0 and (.recognition | length) > 50)
' "$SPEC" > /dev/null

proof_rows=()
proof_count="$(jq '.proof_gates | length' "$SPEC")"
for ((i=0; i<proof_count; i++)); do
  id="$(jq -r ".proof_gates[$i].id" "$SPEC")"
  script="$(jq -r ".proof_gates[$i].script" "$SPEC")"
  gate_spec="$(jq -r ".proof_gates[$i].spec" "$SPEC")"
  summary_rel="$(jq -r ".proof_gates[$i].summary" "$SPEC")"
  proof_out="$OUT/proofs/$id"
  run_log="$OUT/proofs/$id.run.log"
  mkdir -p "$proof_out"
  bash "$script" "$gate_spec" "$proof_out" > "$run_log" 2>&1
  summary_path="$proof_out/$summary_rel"
  test -s "$summary_path"
  jq -n \
    --arg id "$id" \
    --arg script "$script" \
    --arg spec "$gate_spec" \
    --arg run_log "$run_log" \
    --arg summary_path "$summary_path" \
    --slurpfile summary "$summary_path" \
    '{
      id:$id,
      script:$script,
      spec:$spec,
      run_log:$run_log,
      summary_path:$summary_path,
      summary:$summary[0],
      verified:true
    }' > "$proof_out/proof-row.json"
  proof_rows+=("$proof_out/proof-row.json")
done

jq -s '{version:"patchline.contributor-recognition-proofs/v1", proofs:.}' "${proof_rows[@]}" > "$OUT/proofs.json"

score_for() {
  local category="$1"
  local proof_gate="$2"
  case "$category:$proof_gate" in
    new-real-repo-slices:awesome-patchline)
      jq '.proofs[] | select(.id == "awesome-patchline") | .summary.examples' "$OUT/proofs.json"
      ;;
    ecosystem-parsers:awesome-patchline)
      jq '.proofs[] | select(.id == "awesome-patchline") | .summary.ecosystems' "$OUT/proofs.json"
      ;;
    false-positive-reductions:rejected-generated)
      jq '.proofs[] | select(.id == "rejected-generated") | .summary.rejected_interventions' "$OUT/proofs.json"
      ;;
    artifact-improvements:reviewability-examples)
      jq '.proofs[] | select(.id == "reviewability-examples") | .summary.examples' "$OUT/proofs.json"
      ;;
    *)
      echo 1
      ;;
  esac
}

contributor_rows=()
contributor_count="$(jq '.contributors | length' "$SPEC")"
for ((i=0; i<contributor_count; i++)); do
  id="$(jq -r ".contributors[$i].id" "$SPEC")"
  display="$(jq -r ".contributors[$i].display" "$SPEC")"
  category="$(jq -r ".contributors[$i].category" "$SPEC")"
  proof_gate="$(jq -r ".contributors[$i].proof_gate" "$SPEC")"
  badge="$(jq -r ".contributors[$i].badge" "$SPEC")"
  recognition="$(jq -r ".contributors[$i].recognition" "$SPEC")"
  score="$(score_for "$category" "$proof_gate")"
  jq -n \
    --arg id "$id" \
    --arg display "$display" \
    --arg category "$category" \
    --arg proof_gate "$proof_gate" \
    --arg badge "$badge" \
    --arg recognition "$recognition" \
    --argjson score "$score" \
    --slurpfile proofs "$OUT/proofs.json" \
    '{
      id:$id,
      display:$display,
      category:$category,
      proof_gate:$proof_gate,
      badge:$badge,
      recognition:$recognition,
      score:$score,
      proof:($proofs[0].proofs[] | select(.id == $proof_gate)),
      verified:true
    }' > "$OUT/cards/$id.json"
  contributor_rows+=("$OUT/cards/$id.json")
  cat > "$OUT/cards/$id.md" <<EOF
# $display

- Badge: \`$badge\`
- Category: \`$category\`
- Proof gate: \`$proof_gate\`
- Score: \`$score\`

$recognition
EOF
done

jq -s \
  --slurpfile spec "$SPEC" \
  --slurpfile proofs "$OUT/proofs.json" \
  'sort_by(-.score, .id) as $contributors | {
    version:"patchline.contributor-recognition/v1",
    claim:$spec[0].claim,
    proofs:$proofs[0].proofs,
    contributors:$contributors,
    summary:{
      contributors:($contributors | length),
      categories:([$contributors[].category] | unique),
      proof_gates:([$contributors[].proof_gate] | unique),
      badges:([$contributors[].badge] | unique),
      total_score:([$contributors[].score] | add),
      real_repo_slices:($proofs[0].proofs[] | select(.id == "awesome-patchline") | .summary.examples),
      ecosystem_count:($proofs[0].proofs[] | select(.id == "awesome-patchline") | .summary.ecosystems),
      false_positive_reductions:($proofs[0].proofs[] | select(.id == "rejected-generated") | .summary.rejected_interventions),
      artifact_improvements:($proofs[0].proofs[] | select(.id == "reviewability-examples") | .summary.examples),
      verified:all($contributors[]; .verified == true)
    }
  }' "${contributor_rows[@]}" > "$OUT/contributor-recognition.json"

{
  echo "# Patchline contributor recognition"
  echo
  echo "Recognition is generated from public-code proof gates for new real-repo slices, ecosystem parsers, false-positive reductions, and artifact improvements."
  echo
  echo "| Contributor | Category | Badge | Score | Proof gate |"
  echo "| --- | --- | --- | ---: | --- |"
  jq -r '.contributors[] | "| [" + .display + "](cards/" + .id + ".md) | `" + .category + "` | " + .badge + " | " + (.score|tostring) + " | `" + .proof_gate + "` |"' "$OUT/contributor-recognition.json"
  echo
  echo "## Proof summary"
  echo
  jq -r '.summary | "- contributors: `" + (.contributors|tostring) + "`\n- categories: `" + (.categories | join(", ")) + "`\n- real-repo slices: `" + (.real_repo_slices|tostring) + "`\n- ecosystem parsers: `" + (.ecosystem_count|tostring) + "`\n- false-positive reductions: `" + (.false_positive_reductions|tostring) + "`\n- artifact improvements: `" + (.artifact_improvements|tostring) + "`"' "$OUT/contributor-recognition.json"
} > "$OUT/leaderboard.md"

cp "$OUT/leaderboard.md" "$OUT/README.md"
echo "contributor recognition generated: $(jq '.summary.contributors' "$OUT/contributor-recognition.json") contributors, categories $(jq '.summary.categories | length' "$OUT/contributor-recognition.json")"
