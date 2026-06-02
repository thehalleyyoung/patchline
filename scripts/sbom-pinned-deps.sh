#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/sbom-pinned-deps-gate.json}"; OUT="${2:-results/generated/sbom-pinned-deps}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.sbom-pinned-deps-gate/v1"' "$SPEC" > /dev/null
jq '

  .components as $C | .compromised as $X
  | ([ $C[] | select((.version|length>0) and (.hash|length>0)) ]|length) as $pinned
  | ([ $C[] | select(.hash==.installed_hash) ]|length) as $verified
  | {version:"patchline.sbom-pinned-deps/v1",
     total:($C|length), pinned:$pinned, verified_count:$verified,
     all_pinned:($pinned==($C|length)), all_verified:($verified==($C|length)),
     compromise_detected:($X.hash != $X.installed_hash)}

' "$SPEC" > "$OUT/out.json"
{ echo "# SBOM with pinned dependencies"; echo; echo "Components $(jq -r .total "$OUT/out.json"); verified $(jq -r .verified_count "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "sbom-pinned-deps worker: all_verified=$(jq -r .all_verified "$OUT/out.json")"
