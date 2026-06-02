#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/roadmap-board-gate.json}"
OUT="${2:-results/generated/roadmap-board}"
rm -rf "$OUT"
mkdir -p "$OUT/cache" "$OUT/analyses" "$OUT/taxonomy" "$OUT/cards"

jq -e '
  .version == "patchline.roadmap-board-gate/v1" and
  (.claim | length) > 180 and
  (.real_code | length) >= .minimum_public_repos and
  (.planned_features | length) >= .minimum_features and
  all(.real_code[]; (.id | length) > 0 and (.repo | contains("/")) and (.ref | test("^[0-9a-f]{40}$")) and (.subpath | length) > 0) and
  all(.planned_features[]; (.id | test("^[a-z0-9-]+$")) and (.title | length) > 10 and (.gate | startswith("make ")) and (.expected_artifact | length) > 20 and (.why | length) > 50)
' "$SPEC" > /dev/null

analysis_dirs=()
repo_count="$(jq '.real_code | length' "$SPEC")"
for ((i=0; i<repo_count; i++)); do
  id="$(jq -r ".real_code[$i].id" "$SPEC")"
  repo="$(jq -r ".real_code[$i].repo" "$SPEC")"
  ref="$(jq -r ".real_code[$i].ref" "$SPEC")"
  subpath="$(jq -r ".real_code[$i].subpath" "$SPEC")"
  analysis="$OUT/analyses/$id"
  analysis_dirs+=("$analysis")
  go run ./cmd/patchline repo analyze \
    --github "$repo" \
    --ref "$ref" \
    --subpath "$subpath" \
    --download-dir "$OUT/cache" \
    --stages inventory,baseline,propose,compare \
    --proposal-kind all \
    --budget files=4,lines=80,tokens=10000,changes=2 \
    --no-llm \
    --out "$analysis" \
    --json > "$OUT/analyze-$i.json"
done

IFS=,
analyses="${analysis_dirs[*]}"
unset IFS

go run ./cmd/patchline repo taxonomy \
  --analyses "$analyses" \
  --out "$OUT/taxonomy" \
  --json > "$OUT/taxonomy-stdout.json"

feature_rows=()
feature_count="$(jq '.planned_features | length' "$SPEC")"
mode_count="$(jq '.failure_modes | length' "$OUT/taxonomy/failure-taxonomy.json")"
for ((i=0; i<feature_count; i++)); do
  id="$(jq -r ".planned_features[$i].id" "$SPEC")"
  title="$(jq -r ".planned_features[$i].title" "$SPEC")"
  stage="$(jq -r ".planned_features[$i].stage" "$SPEC")"
  owner="$(jq -r ".planned_features[$i].owner" "$SPEC")"
  gate="$(jq -r ".planned_features[$i].gate" "$SPEC")"
  artifact="$(jq -r ".planned_features[$i].expected_artifact" "$SPEC")"
  why="$(jq -r ".planned_features[$i].why" "$SPEC")"
  mode_index=$((i % mode_count))
  mode_file="$OUT/cards/$id.mode.json"
  jq ".failure_modes[$mode_index]" "$OUT/taxonomy/failure-taxonomy.json" > "$mode_file"
  jq -n \
    --arg id "$id" \
    --arg title "$title" \
    --arg stage "$stage" \
    --arg owner "$owner" \
    --arg gate "$gate" \
    --arg artifact "$artifact" \
    --arg why "$why" \
    --slurpfile mode "$mode_file" \
    '{
      id:$id,
      title:$title,
      stage:$stage,
      owner:$owner,
      gate:$gate,
      expected_artifact:$artifact,
      why:$why,
      failure_mode:{
        id:$mode[0].id,
        title:$mode[0].title,
        definition:$mode[0].definition,
        repair_risk:$mode[0].repair_risk,
        maintainer_decision:$mode[0].maintainer_decision,
        occurrences:$mode[0].occurrences,
        public_repos:$mode[0].public_repos,
        evidence_kinds:$mode[0].evidence_kinds,
        example:($mode[0].examples[0])
      },
      expected_artifact_status:"planned",
      verified:true
    }' > "$OUT/cards/$id.json"
  feature_rows+=("$OUT/cards/$id.json")
  cat > "$OUT/cards/$id.md" <<EOF
# $title

- Stage: \`$stage\`
- Owner: \`$owner\`
- Gate: \`$gate\`
- Expected artifact: \`$artifact\`

$why

## Linked real-repo failure mode

$(jq -r '"- Failure mode: `" + .failure_mode.id + "` — " + .failure_mode.title + "\n- Definition: " + .failure_mode.definition + "\n- Repair risk: " + .failure_mode.repair_risk + "\n- Maintainer decision: " + .failure_mode.maintainer_decision + "\n- Example: `" + .failure_mode.example.repo + "` `" + .failure_mode.example.subpath + "` severity `" + .failure_mode.example.severity + "` risk `" + (.failure_mode.example.risk_id // "aggregate-mode-example") + "`"' "$OUT/cards/$id.json")
EOF
done

jq -s \
  --slurpfile taxonomy "$OUT/taxonomy/failure-taxonomy.json" \
  --slurpfile spec "$SPEC" \
  '{
    version:"patchline.roadmap-board/v1",
    claim:$spec[0].claim,
    generated_from:"pinned public-code failure taxonomy",
    taxonomy:{
      public_repos:$taxonomy[0].summary.public_repos,
      failure_modes:$taxonomy[0].summary.failure_modes,
      occurrences:$taxonomy[0].summary.occurrences,
      generated_intervention_links:$taxonomy[0].summary.generated_intervention_links,
      hash:$taxonomy[0].hash
    },
    cards:.,
    summary:{
      features:length,
      planned_features:([.[] | select(.stage == "planned")] | length),
      next_features:([.[] | select(.stage == "next")] | length),
      gates:([.[].gate] | unique),
      expected_artifacts:([.[].expected_artifact] | unique),
      linked_failure_modes:([.[].failure_mode.id] | unique),
      public_repos:$taxonomy[0].summary.public_repos,
      occurrences:$taxonomy[0].summary.occurrences
    }
  }' "${feature_rows[@]}" > "$OUT/roadmap-board.json"

{
  echo "# Patchline public roadmap board"
  echo
  echo "Every planned feature links to a real-repo failure mode, a proof gate, and an expected artifact."
  echo
  echo "| Feature | Stage | Owner | Real failure mode | Gate | Expected artifact |"
  echo "| --- | --- | --- | --- | --- | --- |"
  jq -r '.cards[] | "| [" + .title + "](cards/" + .id + ".md) | " + .stage + " | " + .owner + " | `" + .failure_mode.id + "` from `" + .failure_mode.example.repo + "` | `" + .gate + "` | `" + .expected_artifact + "` |"' "$OUT/roadmap-board.json"
  echo
  echo "## Regenerated evidence"
  echo
  jq -r '.summary | "- features: `" + (.features|tostring) + "`\n- linked failure modes: `" + (.linked_failure_modes | join(", ")) + "`\n- public repositories: `" + (.public_repos|tostring) + "`\n- failure occurrences: `" + (.occurrences|tostring) + "`"' "$OUT/roadmap-board.json"
} > "$OUT/roadmap-board.md"

cp "$OUT/roadmap-board.md" "$OUT/README.md"
echo "roadmap board generated: $(jq '.summary.features' "$OUT/roadmap-board.json") features, modes $(jq '.taxonomy.failure_modes' "$OUT/roadmap-board.json"), repos $(jq '.taxonomy.public_repos' "$OUT/roadmap-board.json")"
