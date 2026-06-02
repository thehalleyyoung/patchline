#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/camera-ready-build-gate.json}"; OUT="${2:-results/generated/camera-ready-build}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.camera-ready-build-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "pinned tooling" "make camera-ready-build-gate"; do grep -F "$phrase" docs/camera-ready-build.md README.md > /dev/null; done
bash scripts/camera-ready-build.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.camera-ready-build/v1" and .pinned==true and .source_driven==true and .floating_pinned==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.camera-ready-build-gate-results/v1",pinned:$r[0].pinned,source_driven:$r[0].source_driven,floating_rejected:($r[0].floating_pinned|not),verified:true}' > "$OUT/gate-summary.json"
echo "camera-ready-build gate passed: camera-ready build pinned and source-driven, floating-version build rejected"
