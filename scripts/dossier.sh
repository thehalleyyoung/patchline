#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/dossier-gate.json}"
OUT="${2:-results/generated/dossier}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.dossier-gate/v1" and (.capabilities|length) >= 1' "$SPEC" > /dev/null

# Certify one capability by checking its full six-artifact evidence chain.
certify() {
  local name="$1"
  local example="examples/${name}-gate.json"
  local worker="scripts/${name}.sh"
  local gate="scripts/${name}-gate.sh"
  local doc="docs/${name}.md"
  local has_example=false has_worker=false has_gate=false has_doc=false has_make=false has_readme=false
  [ -f "$example" ] && has_example=true
  [ -f "$worker" ]  && has_worker=true
  [ -f "$gate" ]    && has_gate=true
  [ -f "$doc" ]     && has_doc=true
  grep -Fq "${name}-gate:" Makefile && has_make=true
  grep -Fq "make ${name}-gate" README.md && has_readme=true
  jq -n --arg n "$name" \
    --argjson e "$has_example" --argjson w "$has_worker" --argjson g "$has_gate" \
    --argjson d "$has_doc" --argjson m "$has_make" --argjson r "$has_readme" \
    '{capability:$n, example:$e, worker:$w, gate:$g, doc:$d, make:$m, readme:$r,
      certified: ($e and $w and $g and $d and $m and $r)}'
}

rows="[]"
while IFS= read -r name; do
  row="$(certify "$name")"
  rows="$(jq --argjson r "$row" '. + [$r]' <<<"$rows")"
done < <(jq -r '.capabilities[]' "$SPEC")

phantom="$(certify "$(jq -r '.phantom_capability' "$SPEC")")"

jq -n --argjson rows "$rows" --argjson phantom "$phantom" '{
  version: "patchline.dossier/v1",
  certified_count: ($rows | map(select(.certified)) | length),
  total: ($rows | length),
  all_certified: ($rows | all(.[]; .certified)),
  uncertified: ($rows | map(select(.certified|not) | .capability)),
  capabilities: $rows,
  phantom: $phantom
}' > "$OUT/dossier.json"

{
  echo "# 1.0 release-readiness dossier"
  echo
  echo "Certified: $(jq -r '.certified_count' "$OUT/dossier.json")/$(jq -r '.total' "$OUT/dossier.json")"
  echo
  echo "Phantom capability certified: $(jq -r '.phantom.certified' "$OUT/dossier.json")"
} > "$OUT/dossier.md"
cp "$OUT/dossier.md" "$OUT/README.md"

echo "dossier worker: certified=$(jq -r '.certified_count' "$OUT/dossier.json")/$(jq -r '.total' "$OUT/dossier.json") phantom=$(jq -r '.phantom.certified' "$OUT/dossier.json")"
