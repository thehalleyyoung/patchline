#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/adoption-case-studies-gate.json}"
OUT="${2:-results/generated/adoption-case-studies}"
rm -rf "$OUT"
mkdir -p "$OUT/studies"

jq -e '.version == "patchline.adoption-case-studies-gate/v1" and (.case_studies|length) >= 3' "$SPEC" > /dev/null

index="$OUT/studies.jsonl"
: > "$index"

n="$(jq '.case_studies | length' "$SPEC")"
for i in $(seq 0 $((n-1))); do
  team="$(jq -r ".case_studies[$i].team" "$SPEC")"
  ctx="$(jq -r ".case_studies[$i].context" "$SPEC")"
  outcome="$(jq -r ".case_studies[$i].outcome" "$SPEC")"
  ncap="$(jq ".case_studies[$i].capabilities | length" "$SPEC")"
  slug="$(printf '%s' "$team" | tr '[:upper:] ' '[:lower:]-' | tr -cd 'a-z0-9-')"

  study="$OUT/studies/${slug}.md"
  {
    echo "# Case study: ${team}"
    echo
    echo "**Integration context:** ${ctx}"
    echo
    echo "## Capabilities used"
    echo
    jq -r ".case_studies[$i].capabilities[] | \"- \`make \" + . + \"\`\"" "$SPEC"
    echo
    echo "## Outcome"
    echo
    echo "$outcome"
  } > "$study"

  caps_json="$(jq ".case_studies[$i].capabilities" "$SPEC")"
  jq -n --arg team "$team" --arg ctx "$ctx" --arg slug "$slug" --argjson ncap "$ncap" \
    --argjson caps "$caps_json" -c \
    '{team:$team, context:$ctx, slug:$slug, capabilities:$caps, capability_count:$ncap}' >> "$index"
done

sort -o "$index" "$index"

{
  echo "# Adoption case studies"
  echo
  echo "Teams using Patchline alongside CI, observability, and migration tooling."
  echo
  echo "| Team | Context | Capabilities |"
  echo "|---|---|---|"
  jq -r '"| \(.team) | `\(.context)` | \(.capability_count) |"' "$index"
} > "$OUT/adoption-case-studies.md"
cp "$OUT/adoption-case-studies.md" "$OUT/README.md"

jq -s '{
  version: "patchline.adoption-case-studies/v1",
  studies: sort_by(.team),
  contexts: ([.[].context] | unique | sort),
  count: length
}' "$index" > "$OUT/adoption-case-studies.json"

echo "adoption-case-studies worker: $(jq -r .count "$OUT/adoption-case-studies.json") studies across $(jq -r '.contexts|length' "$OUT/adoption-case-studies.json") contexts"
