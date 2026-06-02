#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/shell-portability-gate.json}"
OUT="${2:-results/generated/shell-portability}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.shell-portability-gate/v1" and (.clean_scripts|length) >= 1' "$SPEC" > /dev/null

scan_file() {
  local f="$1"
  grep -Eq '(^|[^_])mapfile ' "$f" 2>/dev/null && echo "mapfile" || true
  grep -Eq '> *<?/tmp/' "$f" 2>/dev/null && echo "tmp_write" || true
  if grep -Eq 'sed -i ' "$f" 2>/dev/null && ! grep -Eq "sed -i ''" "$f" 2>/dev/null; then
    echo "gnu_sed_no_suffix"
  fi
}

hits_json_for() {
  scan_file "$1" | jq -R 'select(length>0)' | jq -cs '.'
}

results="[]"
while IFS= read -r f; do
  h="$(hits_json_for "$f")"
  results="$(jq --arg f "$f" --argjson h "$h" '. + [{script:$f, hazards:$h, clean: (($h|length)==0)}]' <<<"$results")"
done < <(jq -r '.clean_scripts[]' "$SPEC")

neg="$(jq -r '.negative_control' "$SPEC")"
neg_json="$(hits_json_for "$neg")"

jq -n \
  --argjson results "$results" \
  --arg neg "$neg" \
  --argjson neg_hits "$neg_json" '{
  version: "patchline.shell-portability/v1",
  clean_scripts: $results,
  all_clean: ($results | all(.[]; .clean)),
  negative_control: { script: $neg, hazards: $neg_hits, flagged: (($neg_hits|length) > 0) }
}' > "$OUT/shell-portability.json"

{
  echo "# Shell portability lint"
  echo
  echo "All shipped scripts clean: $(jq -r '.all_clean' "$OUT/shell-portability.json")"
  echo
  echo "Negative control hazards: $(jq -rc '.negative_control.hazards' "$OUT/shell-portability.json")"
} > "$OUT/shell-portability.md"
cp "$OUT/shell-portability.md" "$OUT/README.md"

echo "shell-portability worker: all_clean=$(jq -r '.all_clean' "$OUT/shell-portability.json") neg_hazards=$(jq -r '.negative_control.hazards|length' "$OUT/shell-portability.json")"
