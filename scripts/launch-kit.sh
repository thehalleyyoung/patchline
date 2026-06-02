#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/launch-kit-gate.json}"
OUT="${2:-results/generated/launch-kit}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.launch-kit-gate/v1" and (.channels|length) >= 1' "$SPEC" > /dev/null

jq '
  .char_limit as $lim
  | def check($c): {
      name: $c.name,
      kind: $c.kind,
      length: ($c.text | length),
      present: (($c.text // "" | length) > 0),
      within_limit: (if $c.kind == "social" then (($c.text | length) <= $lim) else true end)
    };
  {
    version: "patchline.launch-kit/v1",
    char_limit: $lim,
    channels: [.channels[] | check(.)],
    required: ["readme-hook", "long-post", "social-thread", "demo-script", "faq"]
  }
  | .all_present = ([.required[] as $r | (.channels | map(.name) | index($r)) != null] | all)
  | .all_within_limit = (.channels | all(.[]; .within_limit and .present))
  | .launch_ready = (.all_present and .all_within_limit)
' "$SPEC" > "$OUT/launch-kit.json"

# Evaluate negative control independently.
NEG="$(jq '.char_limit as $lim | .negative_control | {name, kind, length: (.text|length), within_limit: ((.text|length) <= $lim)}' "$SPEC")"
jq --argjson neg "$NEG" '. + {negative_control: $neg}' "$OUT/launch-kit.json" > "$OUT/launch-kit.tmp" && mv "$OUT/launch-kit.tmp" "$OUT/launch-kit.json"

{
  echo "# Star-growth launch kit"
  echo
  echo "Launch ready: $(jq -r '.launch_ready' "$OUT/launch-kit.json")"
  echo
  echo "Negative-control post within limit: $(jq -r '.negative_control.within_limit' "$OUT/launch-kit.json")"
} > "$OUT/launch-kit.md"
cp "$OUT/launch-kit.md" "$OUT/README.md"

echo "launch-kit worker: launch_ready=$(jq -r '.launch_ready' "$OUT/launch-kit.json") neg_within=$(jq -r '.negative_control.within_limit' "$OUT/launch-kit.json")"
