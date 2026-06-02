#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/contributor-badges-gate.json}"
OUT="${2:-results/generated/contributor-badges}"
rm -rf "$OUT"
mkdir -p "$OUT/badges"

jq -e '.version == "patchline.contributor-badges-gate/v1" and (.contributors|length) >= 1 and (.tiers|length) >= 1' "$SPEC" > /dev/null

badges="$OUT/badges.jsonl"
: > "$badges"

n="$(jq '.contributors | length' "$SPEC")"
for i in $(seq 0 $((n-1))); do
  handle="$(jq -r ".contributors[$i].handle" "$SPEC")"
  # Keep only contributions whose gate script actually exists.
  backed=()
  while IFS= read -r cap; do
    [ -n "$cap" ] || continue
    if [ -f "scripts/${cap}-gate.sh" ]; then
      backed+=("$cap")
    fi
  done < <(jq -r ".contributors[$i].contributions[]" "$SPEC")
  count="${#backed[@]}"

  # Deterministic tier: highest tier whose threshold is met.
  tier="none"
  while IFS= read -r t; do
    tname="$(jq -r '.name' <<< "$t")"
    tmin="$(jq -r '.min' <<< "$t")"
    if [ "$count" -ge "$tmin" ]; then tier="$tname"; fi
  done < <(jq -c '.tiers | sort_by(.min)[]' "$SPEC")

  backed_json="$(printf '%s\n' "${backed[@]}" | jq -R . | jq -s 'map(select(length>0)) | sort')"

  jq -n --arg handle "$handle" --argjson count "$count" --arg tier "$tier" \
    --argjson backed "$backed_json" -c \
    '{handle:$handle, backed_contributions:$backed, count:$count, tier:$tier}' >> "$badges"

  # Shields-style endpoint JSON per contributor.
  jq -n --arg label "patchline" --arg message "$tier ($count)" --arg color "blue" \
    '{schemaVersion:1, label:$label, message:$message, color:$color}' \
    > "$OUT/badges/${handle}.json"
done

sort -o "$badges" "$badges"

{
  echo "# Patchline contributor wall of fame"
  echo
  echo "Recognition tiers are computed only from **gate-backed** contributions."
  echo
  echo "| Contributor | Tier | Gate-backed contributions |"
  echo "|---|---|---|"
  jq -r '"| `\(.handle)` | \(.tier) | \(.count) |"' "$badges"
} > "$OUT/badges.md"
cp "$OUT/badges.md" "$OUT/README.md"

jq -s '{
  version: "patchline.contributor-badges/v1",
  contributors: length,
  badges: sort_by(.handle)
}' "$badges" > "$OUT/contributor-badges.json"

echo "contributor-badges worker: $(jq -r .contributors "$OUT/contributor-badges.json") contributors scored from gate-backed data"
