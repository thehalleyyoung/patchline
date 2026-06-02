#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/ecosystem-certification-gate.json}"; OUT="${2:-results/generated/ecosystem-certification}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.ecosystem-certification-gate/v1"' "$SPEC" > /dev/null
jq '

  .required_checks as $R | .extension as $E | .bad_extension as $B
  | ([ $R[] | . as $c | ($E.passed_checks|index($c))!=null ]|all) as $certified
  | ([ $R[] | . as $c | ($B.passed_checks|index($c))!=null ]|all) as $bcertified
  | {version:"patchline.ecosystem-certification/v1",
     required:($R|length),
     certified:$certified,
     bad_certified:$bcertified}

' "$SPEC" > "$OUT/out.json"
{ echo "# Extension ecosystem certification"; echo; echo "Certified $(jq -r .certified "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "ecosystem-certification worker: certified=$(jq -r .certified "$OUT/out.json")"
