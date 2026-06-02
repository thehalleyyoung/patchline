#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/anonymized-build-gate.json}"; OUT="${2:-results/generated/anonymized-build}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.anonymized-build-gate/v1"' "$SPEC" > /dev/null
jq '

  .identifying_tokens as $T | .anonymized_content as $a | .raw_content as $raw
  | ([ $T[] | select(. as $t | $a | contains($t)) ]|length) as $leaks
  | ([ $T[] | select(. as $t | $raw | contains($t)) ]|length) as $rawleaks
  | {version:"patchline.anonymized-build/v1",
     leaks:$leaks, clean:($leaks==0),
     raw_leaks:$rawleaks, raw_detected:($rawleaks>0)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Anonymized-for-review build"; echo; echo "Leaks $(jq -r .leaks "$OUT/out.json"); clean $(jq -r .clean "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "anonymized-build worker: clean=$(jq -r .clean "$OUT/out.json") raw_detected=$(jq -r .raw_detected "$OUT/out.json")"
