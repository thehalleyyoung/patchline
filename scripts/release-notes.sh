#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/release-notes-gate.json}"
OUT="${2:-results/generated/release-notes}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.release-notes-gate/v1"' "$SPEC" > /dev/null
release="$(jq -r '.release' "$SPEC")"

# Live gate set: every scripts/*-gate.sh present today.
ls scripts/*-gate.sh 2>/dev/null | sed 's#scripts/##; s#\.sh$##' | sort > "$OUT/.current"
jq -r '.previous_release_gates[]' "$SPEC" | sort > "$OUT/.previous"

# Public proof delta = gates present now but not in the previous release.
comm -23 "$OUT/.current" "$OUT/.previous" > "$OUT/.added"
added_count="$(wc -l < "$OUT/.added" | tr -d ' ')"

{
  echo "# Release notes: ${release}"
  echo
  echo "## New public proofs (gate delta)"
  echo
  echo "These gates are present in this release but were not in the previous one."
  echo "Each is reproducible against real, pinned public code."
  echo
  if [ "$added_count" -gt 0 ]; then
    while IFS= read -r g; do
      [ -n "$g" ] || continue
      echo "- \`make ${g}\`"
    done < "$OUT/.added"
  else
    echo "- (no new gates this release)"
  fi
  echo
  echo "## Contributor recognition"
  echo
  jq -r '.recognition[] | "- @\(.handle): \(.highlight)"' "$SPEC"
  echo
  echo "## Known limitations"
  echo
  jq -r '.known_limitations[] | "- " + .' "$SPEC"
} > "$OUT/release-notes.md"
cp "$OUT/release-notes.md" "$OUT/README.md"

# Machine-readable summary; the added list is sorted for determinism.
added_json="$(jq -R . "$OUT/.added" | jq -s 'map(select(length>0)) | sort')"
jq -n \
  --arg release "$release" \
  --argjson added "$added_json" \
  --argjson recognition "$(jq '.recognition' "$SPEC")" \
  --argjson limitations "$(jq '.known_limitations' "$SPEC")" '
  {
    version: "patchline.release-notes/v1",
    release: $release,
    added_gates: $added,
    added_count: ($added | length),
    recognition_count: ($recognition | length),
    limitations_count: ($limitations | length)
  }' > "$OUT/release-notes.json"

echo "release-notes worker: ${release} with $(jq -r .added_count "$OUT/release-notes.json") new proof(s)"
