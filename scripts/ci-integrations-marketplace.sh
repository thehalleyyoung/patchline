#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/ci-integrations-marketplace-gate.json}"; OUT="${2:-results/generated/ci-integrations-marketplace}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.ci-integrations-marketplace-gate/v1"' "$SPEC" > /dev/null
jq '

  def r4: (.*10000|round)/10000;
  .integrations as $I | .unverified_listing as $U
  | ([ $I[] | select(.verified and ((.recipe|length)>0)) ]|length) as $ok
  | {version:"patchline.ci-integrations-marketplace/v1",
     integrations:($I|length), verified:$ok,
     verified_rate:(($ok/($I|length))|r4),
     all_verified:($ok==($I|length)),
     unverified_ok:($U.verified and (($U.recipe|length)>0))}

' "$SPEC" > "$OUT/out.json"
{ echo "# CI integrations marketplace listing"; echo; echo "Verified $(jq -r .verified "$OUT/out.json")/$(jq -r .integrations "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "ci-integrations-marketplace worker: all_verified=$(jq -r .all_verified "$OUT/out.json")"
