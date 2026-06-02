#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/reproducible-build-attestation-gate.json}"; OUT="${2:-results/generated/reproducible-build-attestation}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.reproducible-build-attestation-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "attestation" "make reproducible-build-attestation-gate"; do grep -F "$phrase" docs/reproducible-build-attestation.md README.md > /dev/null; done
bash scripts/reproducible-build-attestation.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.reproducible-build-attestation/v1" and .pinned==true and .reproducible==true and .nondeterministic_reproducible==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.reproducible-build-attestation-gate-results/v1",reproducible:$r[0].reproducible,nondeterminism_flagged:($r[0].nondeterministic_reproducible|not),verified:true}' > "$OUT/gate-summary.json"
echo "reproducible-build-attestation gate passed: two pinned builds byte-identical, nondeterministic build flagged"
