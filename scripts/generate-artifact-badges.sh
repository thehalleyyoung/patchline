#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/artifact-badges-gate.json}"
OUT="${2:-results/generated/artifact-badges}"
rm -rf "$OUT"
mkdir -p "$OUT/badges" "$OUT/evidence"

jq -e '
  . as $root |
  .version == "patchline.artifact-badges-gate/v1" and
  (.claim | length) > 120 and
  (.required_badges | length) == 4 and
  (.badges | length) == (.required_badges | length) and
  all(.required_badges[]; . as $id | any($root.badges[]; .id == $id)) and
  all(.badges[]; (.id | test("^[a-z0-9-]+$")) and (.label | length) > 0 and (.message | length) > 0 and (.color | test("^[0-9a-f]{6}$")) and (.criteria | length) >= $root.minimum_justifications_per_badge)
' "$SPEC" > /dev/null

profile_spec="$(jq -r '.evidence_profile_spec' "$SPEC")"
bash scripts/artifact-container-rebuild.sh "$profile_spec" "$OUT/evidence/artifact-container-rebuild" > "$OUT/evidence/artifact-container-rebuild.run.log"

svg_badge() {
  local label="$1" message="$2" color="$3" file="$4"
  local label_width=$(( ${#label} * 7 + 20 ))
  local message_width=$(( ${#message} * 7 + 20 ))
  local total_width=$(( label_width + message_width ))
  cat > "$file" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="$total_width" height="20" role="img" aria-label="$label: $message">
  <title>$label: $message</title>
  <linearGradient id="s" x2="0" y2="100%">
    <stop offset="0" stop-color="#bbb" stop-opacity=".1"/>
    <stop offset="1" stop-opacity=".1"/>
  </linearGradient>
  <clipPath id="r"><rect width="$total_width" height="20" rx="3" fill="#fff"/></clipPath>
  <g clip-path="url(#r)">
    <rect width="$label_width" height="20" fill="#555"/>
    <rect x="$label_width" width="$message_width" height="20" fill="#$color"/>
    <rect width="$total_width" height="20" fill="url(#s)"/>
  </g>
  <g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" font-size="11">
    <text x="$(( label_width / 2 ))" y="15" fill="#010101" fill-opacity=".3">$label</text>
    <text x="$(( label_width / 2 ))" y="14">$label</text>
    <text x="$(( label_width + message_width / 2 ))" y="15" fill="#010101" fill-opacity=".3">$message</text>
    <text x="$(( label_width + message_width / 2 ))" y="14">$message</text>
  </g>
</svg>
SVG
}

badge_rows=()
count="$(jq '.badges | length' "$SPEC")"
for ((i=0; i<count; i++)); do
  id="$(jq -r ".badges[$i].id" "$SPEC")"
  label="$(jq -r ".badges[$i].label" "$SPEC")"
  message="$(jq -r ".badges[$i].message" "$SPEC")"
  color="$(jq -r ".badges[$i].color" "$SPEC")"
  svg="$OUT/badges/$id.svg"
  row="$OUT/badge-$id.json"
  svg_badge "$label" "$message" "$color" "$svg"
  jq -n \
    --arg id "$id" \
    --arg label "$label" \
    --arg message "$message" \
    --arg color "$color" \
    --arg svg "$svg" \
    --slurpfile criteria <(jq ".badges[$i].criteria" "$SPEC") \
    '{
      id:$id,
      label:$label,
      message:$message,
      color:$color,
      svg:$svg,
      markdown:"![\($label)-\($message)](\($svg))",
      criteria:$criteria[0],
      awarded:true
    }' > "$row"
  badge_rows+=("$row")
done

jq -n \
  --slurpfile badges <(jq -s '.' "${badge_rows[@]}") \
  --slurpfile rebuild "$OUT/evidence/artifact-container-rebuild/rebuild-summary.json" \
  '{
    version:"patchline.artifact-badges/v1",
    badges:$badges[0],
    evidence:{
      profile:"evidence/artifact-container-rebuild/rebuild-summary.json",
      public_repos:$rebuild[0].summary.public_repos,
      ranked_risks:$rebuild[0].summary.ranked_risks,
      generated_files:$rebuild[0].summary.generated_files,
      rejected_examples:$rebuild[0].summary.rejected_examples,
      evidence_artifact_types:$rebuild[0].summary.evidence_artifact_types,
      verified:$rebuild[0].summary.verified
    },
    summary:{
      badges:($badges[0] | length),
      awarded:($badges[0] | map(select(.awarded == true)) | length),
      public_repos:$rebuild[0].summary.public_repos,
      ranked_risks:$rebuild[0].summary.ranked_risks,
      generated_files:$rebuild[0].summary.generated_files,
      rejected_examples:$rebuild[0].summary.rejected_examples,
      verified:($rebuild[0].summary.verified == true and all($badges[0][]; .awarded == true))
    }
  }' > "$OUT/artifact-badges.json"

{
  echo "# Patchline artifact badges"
  echo
  echo "These badges are awarded from gate-backed evidence, not from unchecked claims."
  echo
  echo "## Badge strip"
  echo
  jq -r '.badges[] | "[![" + .label + "-" + .message + "](" + .svg + ")](#" + .id + ")"' "$OUT/artifact-badges.json" | paste -sd' ' -
  echo
  echo
  echo "## Gate-backed justifications"
  echo
  jq -r '.badges[] | "### " + .message + "\n\n" + (.criteria | map("- " + .) | join("\n")) + "\n"' "$OUT/artifact-badges.json"
  echo "## Public-code evidence"
  echo
  jq -r '.evidence | "- public repositories: `" + (.public_repos|tostring) + "`\n- ranked risks: `" + (.ranked_risks|tostring) + "`\n- generated files: `" + (.generated_files|tostring) + "`\n- rejected bad-output examples: `" + (.rejected_examples|tostring) + "`\n- evidence artifact types: `" + (.evidence_artifact_types|tostring) + "`"' "$OUT/artifact-badges.json"
} > "$OUT/badges.md"

cp "$OUT/badges.md" "$OUT/README.md"
echo "artifact badges generated: badges $(jq '.summary.badges' "$OUT/artifact-badges.json"), repos $(jq '.summary.public_repos' "$OUT/artifact-badges.json"), risks $(jq '.summary.ranked_risks' "$OUT/artifact-badges.json")"
