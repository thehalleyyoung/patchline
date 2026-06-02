#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/artifact-mirror-gate.json}"
OUT="${2:-results/generated/artifact-mirror}"
rm -rf "$OUT"
mkdir -p "$OUT/candidates" "$OUT/mirror"

jq -e '.version == "patchline.artifact-mirror-gate/v1" and (.artifacts|length) >= 1 and (.markers|length) >= 1' "$SPEC" > /dev/null

# Materialize candidate artifacts.
while IFS= read -r a; do
  name="$(jq -r '.name' <<<"$a")"
  jq -r '.content' <<<"$a" > "$OUT/candidates/$name"
done < <(jq -c '.artifacts[]' "$SPEC")

# Scan each candidate for sensitivity markers; mirror clean ones, quarantine the rest.
mirrored="[]"
quarantined="[]"
while IFS= read -r name; do
  f="$OUT/candidates/$name"
  hit=""
  while IFS= read -r m; do
    if grep -Fq "$m" "$f"; then hit="$m"; break; fi
  done < <(jq -r '.markers[]' "$SPEC")
  if [ -z "$hit" ]; then
    cp "$f" "$OUT/mirror/$name"
    h="$(shasum -a 256 "$f" | cut -d' ' -f1)"
    mirrored="$(jq --arg n "$name" --arg h "$h" '. + [{name:$n, checksum:$h}]' <<<"$mirrored")"
  else
    quarantined="$(jq --arg n "$name" --arg m "$hit" '. + [{name:$n, marker:$m}]' <<<"$quarantined")"
  fi
done < <(jq -r '.artifacts[].name' "$SPEC")

jq -n --argjson mirrored "$mirrored" --argjson quarantined "$quarantined" '{
  version: "patchline.artifact-mirror/v1",
  mirrored: $mirrored,
  quarantined: $quarantined,
  mirrored_count: ($mirrored | length),
  quarantined_count: ($quarantined | length)
}' > "$OUT/artifact-mirror.json"

{
  echo "# Public artifact mirror"
  echo
  echo "Mirrored (public): $(jq -rc '[.mirrored[].name]' "$OUT/artifact-mirror.json")"
  echo
  echo "Quarantined (sensitive): $(jq -rc '[.quarantined[] | {name, marker}]' "$OUT/artifact-mirror.json")"
} > "$OUT/artifact-mirror.md"
cp "$OUT/artifact-mirror.md" "$OUT/README.md"

echo "artifact-mirror worker: mirrored=$(jq -r .mirrored_count "$OUT/artifact-mirror.json") quarantined=$(jq -r .quarantined_count "$OUT/artifact-mirror.json")"
