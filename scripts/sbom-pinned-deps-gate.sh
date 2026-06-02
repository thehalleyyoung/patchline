#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/sbom-pinned-deps-gate.json}"; OUT="${2:-results/generated/sbom-pinned-deps}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.sbom-pinned-deps-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "supply-chain" "make sbom-pinned-deps-gate"; do grep -F "$phrase" docs/sbom-pinned-deps.md README.md > /dev/null; done
bash scripts/sbom-pinned-deps.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.sbom-pinned-deps/v1" and .all_pinned==true and .all_verified==true and .compromise_detected==true' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.sbom-pinned-deps-gate-results/v1",all_verified:$r[0].all_verified,compromise_detected:$r[0].compromise_detected,verified:true}' > "$OUT/gate-summary.json"
echo "sbom-pinned-deps gate passed: all deps pinned and verified, compromised hash flagged"
