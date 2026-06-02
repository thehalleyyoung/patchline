#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/starter-issues-gate.json}"
OUT="${2:-results/generated/starter-issues}"
rm -rf "$OUT"
mkdir -p "$OUT/issues"

jq -e '.version == "patchline.starter-issues-gate/v1" and (.cards|length) >= 1' "$SPEC" > /dev/null

index="$OUT/issues.jsonl"
: > "$index"

n="$(jq '.cards | length' "$SPEC")"
for i in $(seq 0 $((n-1))); do
  title="$(jq -r ".cards[$i].title" "$SPEC")"
  fm="$(jq -r ".cards[$i].failure_mode" "$SPEC")"
  gate="$(jq -r ".cards[$i].expected_gate" "$SPEC")"
  ap="$(jq -r ".cards[$i].artifact_path" "$SPEC")"
  slug="$(printf '%s' "$title" | tr '[:upper:] ' '[:lower:]-' | tr -cd 'a-z0-9-' | cut -c1-60)"
  gate_exists=false; [ -f "scripts/${gate}.sh" ] && gate_exists=true

  issue="$OUT/issues/${slug}.md"
  {
    echo "# $title"
    echo
    echo "**Labels:** good first issue, gate-backed"
    echo
    echo "## Context"
    echo
    echo "This issue is generated from a roadmap card. Completing it closes a concrete"
    echo "gap proven by a reproducible gate."
    echo
    echo "## Failure mode prevented"
    echo
    echo "$fm"
    echo
    echo "## Expected gate"
    echo
    echo "Make \`make ${gate}\` pass with your change. (gate present today: ${gate_exists})"
    echo
    echo "## Artifact path"
    echo
    echo "Generated evidence lands under \`${ap}\`."
    echo
    echo "## Acceptance criteria"
    echo
    echo "- [ ] A real, pinned public repository demonstrates the new behavior."
    echo "- [ ] A unit test covers a positive case and a no-false-positive case."
    echo "- [ ] \`make ${gate}\` passes and writes \`${ap}/gate-summary.json\`."
    echo "- [ ] Docs and README mention stay in sync with the gate."
    echo
    echo "## Reproduce"
    echo
    echo '```'
    echo "make ${gate}"
    echo '```'
  } > "$issue"

  jq -n --arg title "$title" --arg slug "$slug" --arg fm "$fm" --arg gate "$gate" \
    --arg ap "$ap" --argjson gate_exists "$gate_exists" -c \
    '{title:$title, slug:$slug, failure_mode:$fm, expected_gate:$gate, artifact_path:$ap, gate_exists:$gate_exists}' \
    >> "$index"
done

sort -o "$index" "$index"

{
  echo "# Generated starter issues"
  echo
  echo "Each issue is generated from a roadmap card and is tied to a gate from day one."
  echo
  echo "| Issue | Expected gate | Artifact path |"
  echo "|---|---|---|"
  jq -r '"| \(.title) | `make \(.expected_gate)` | `\(.artifact_path)` |"' "$index"
} > "$OUT/starter-issues.md"
cp "$OUT/starter-issues.md" "$OUT/README.md"

jq -s '{
  version: "patchline.starter-issues/v1",
  issues: sort_by(.slug),
  count: length
}' "$index" > "$OUT/starter-issues.json"

echo "starter-issues worker: $(jq -r .count "$OUT/starter-issues.json") issues generated from roadmap cards"
