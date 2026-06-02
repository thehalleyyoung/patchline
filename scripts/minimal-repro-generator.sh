#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/minimal-repro-generator-gate.json}"; OUT="${2:-results/generated/minimal-repro-generator}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.minimal-repro-generator-gate/v1"' "$SPEC" > /dev/null
jq '

  {version:"patchline.minimal-repro-generator/v1",
   original_size:.original_size, reduced_size:.reduced_size,
   smaller:(.reduced_size < .original_size),
   verdict_preserved:(.reduced_verdict == .original_verdict),
   minimal:.minimal,
   over_reduced_preserved:(.over_reduced.verdict == .original_verdict)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Minimal-reproduction generator"; echo; echo "Reduced $(jq -r .original_size "$OUT/out.json") -> $(jq -r .reduced_size "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "minimal-repro-generator worker: smaller=$(jq -r .smaller "$OUT/out.json") verdict_preserved=$(jq -r .verdict_preserved "$OUT/out.json")"
