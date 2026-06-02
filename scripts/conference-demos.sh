#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/conference-demos-gate.json}"
OUT="${2:-results/generated/conference-demos}"
rm -rf "$OUT"
mkdir -p "$OUT/scripts"

jq -e '.version == "patchline.conference-demos-gate/v1" and (.demos|length) >= 4' "$SPEC" > /dev/null

index="$OUT/demos.jsonl"
: > "$index"

n="$(jq '.demos | length' "$SPEC")"
for i in $(seq 0 $((n-1))); do
  audience="$(jq -r ".demos[$i].audience" "$SPEC")"
  focus="$(jq -r ".demos[$i].focus" "$SPEC")"
  nsteps="$(jq ".demos[$i].steps | length" "$SPEC")"
  slug="$(printf '%s' "$audience" | tr '[:upper:] ' '[:lower:]-' | tr -cd 'a-z0-9-')"

  script="$OUT/scripts/${slug}.md"
  {
    echo "# Conference demo: ${audience}"
    echo
    echo "**Focus:** ${focus}"
    echo
    echo "## Run sheet"
    echo
    step_no=1
    while IFS= read -r g; do
      echo "${step_no}. \`make ${g}\` — reproducible, ~1 min. Talking point: what this proves."
      step_no=$((step_no+1))
    done < <(jq -r ".demos[$i].steps[]" "$SPEC")
    echo
    echo "_Every step is a gate; if it is green on main, it is green on stage._"
  } > "$script"

  steps_json="$(jq ".demos[$i].steps" "$SPEC")"
  jq -n --arg audience "$audience" --arg slug "$slug" --argjson nsteps "$nsteps" \
    --argjson steps "$steps_json" -c \
    '{audience:$audience, slug:$slug, steps:$steps, step_count:$nsteps}' >> "$index"
done

sort -o "$index" "$index"

{
  echo "# Conference demo scripts"
  echo
  echo "Audience-tailored, gate-backed run sheets."
  echo
  echo "| Audience | Steps |"
  echo "|---|---|"
  jq -r '"| `\(.audience)` | \(.step_count) |"' "$index"
} > "$OUT/conference-demos.md"
cp "$OUT/conference-demos.md" "$OUT/README.md"

jq -s '{
  version: "patchline.conference-demos/v1",
  demos: sort_by(.audience),
  count: length
}' "$index" > "$OUT/conference-demos.json"

echo "conference-demos worker: $(jq -r .count "$OUT/conference-demos.json") audience demos generated"
