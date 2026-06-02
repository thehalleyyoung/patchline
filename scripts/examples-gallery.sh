#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/examples-gallery-gate.json}"
OUT="${2:-results/generated/examples-gallery}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.examples-gallery-gate/v1"' "$SPEC" > /dev/null
min_entries="$(jq -r '.min_entries' "$SPEC")"

# Build a gallery entry for every example spec that carries a claim. An entry is
# "backed" when a matching gate script and capability doc both exist.
entries_json="$OUT/entries.jsonl"
: > "$entries_json"
total=0
backed=0
orphans=0

for spec in examples/*.json; do
  base="$(basename "$spec" .json)"
  claim="$(jq -r '.claim // empty' "$spec")"
  [ -n "$claim" ] || continue
  total=$((total+1))
  # Capability slug: strip a trailing -gate to find the doc/worker pair.
  cap="${base%-gate}"
  gate_script="scripts/${base}.sh"
  doc="docs/${cap}.md"
  has_gate=false; [ -f "$gate_script" ] && has_gate=true
  has_doc=false; [ -f "$doc" ] && has_doc=true
  real_repo="$(jq -r '.real_repo // empty' "$spec")"
  if [ "$has_gate" = true ] && [ "$has_doc" = true ]; then
    backed=$((backed+1))
    jq -n \
      --arg id "$base" \
      --arg cap "$cap" \
      --arg claim "$claim" \
      --arg real_repo "$real_repo" \
      --argjson has_gate "$has_gate" \
      --argjson has_doc "$has_doc" \
      -c '{id:$id, capability:$cap, claim:$claim, real_repo:$real_repo, has_gate:$has_gate, has_doc:$has_doc}' \
      >> "$entries_json"
  else
    orphans=$((orphans+1))
  fi
done

# Deterministic, sorted gallery index.
sort -o "$entries_json" "$entries_json"

# Render Markdown gallery.
{
  echo "# Patchline Examples Gallery"
  echo
  echo "Generated from \`examples/*.json\` specifications and their gate/doc backing."
  echo "Each entry advertises only what a real example spec and a reproducible gate prove."
  echo
  echo "| Capability | Backed | Pinned real repo |"
  echo "|---|---|---|"
  while IFS= read -r line; do
    cap="$(jq -r '.capability' <<< "$line")"
    backed_flag="$(jq -r 'if (.has_gate and .has_doc) then "yes" else "no" end' <<< "$line")"
    rr="$(jq -r '.real_repo // "" | if . == "" then "—" else . end' <<< "$line")"
    echo "| \`$cap\` | $backed_flag | $rr |"
  done < "$entries_json"
} > "$OUT/gallery.md"

# Render a minimal static HTML index (no analytics, no external assets).
{
  echo "<!doctype html><html><head><meta charset=\"utf-8\">"
  echo "<title>Patchline Examples Gallery</title></head><body>"
  echo "<h1>Patchline Examples Gallery</h1>"
  echo "<p>$total capability examples, $backed gate-backed.</p><ul>"
  while IFS= read -r line; do
    cap="$(jq -r '.capability' <<< "$line")"
    echo "<li><code>$cap</code></li>"
  done < "$entries_json"
  echo "</ul></body></html>"
} > "$OUT/index.html"

jq -n \
  --argjson total "$total" \
  --argjson backed "$backed" \
  --argjson orphans "$orphans" \
  --argjson min_entries "$min_entries" \
  --slurpfile entries <(jq -s '.' "$entries_json") '
  {
    version: "patchline.examples-gallery/v1",
    total_specs: $total,
    listed_entries: $backed,
    excluded_specs: $orphans,
    min_entries: $min_entries,
    sorted: true,
    entries: $entries[0]
  }' > "$OUT/examples-gallery.json"

cp "$OUT/gallery.md" "$OUT/README.md"

echo "examples-gallery worker: $total specs, $backed listed (gate+doc backed), $orphans excluded"
