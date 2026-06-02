#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/governance-policy-gate.json}"; OUT="${2:-results/generated/governance-policy}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.governance-policy-gate/v1"' "$SPEC" > /dev/null
jq '

  .min_deprecation_days as $min | .changes as $C | .rushed_change as $X
  | (.sem_version|test("^[0-9]+\\.[0-9]+\\.[0-9]+$")) as $semver
  | ([ $C[] | select(.breaking) | (.deprecation_days >= $min) and ((.adr|length)>0) ]|all) as $ok
  | {version:"patchline.governance-policy/v1",
     semver:$semver, breaking_compliant:$ok,
     rushed_compliant:(($X.deprecation_days >= $min) and (($X.adr|length)>0) )}

' "$SPEC" > "$OUT/out.json"
{ echo "# Governance and versioning policy"; echo; echo "Semver $(jq -r .semver "$OUT/out.json"); breaking compliant $(jq -r .breaking_compliant "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "governance-policy worker: breaking_compliant=$(jq -r .breaking_compliant "$OUT/out.json")"
