#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/camera-ready-build-gate.json}"; OUT="${2:-results/generated/camera-ready-build}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.camera-ready-build-gate/v1"' "$SPEC" > /dev/null
jq '

  {version:"patchline.camera-ready-build/v1",
   tex_version:.tex_version,
   pinned:(.pinned and (.tex_version != "latest")),
   source_driven:((.source|length)>0),
   floating_pinned:(.floating_build.pinned and (.floating_build.tex_version != "latest"))}

' "$SPEC" > "$OUT/out.json"
{ echo "# Camera-ready build pipeline"; echo; echo "TeX $(jq -r .tex_version "$OUT/out.json"); pinned $(jq -r .pinned "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "camera-ready-build worker: pinned=$(jq -r .pinned "$OUT/out.json")"
