#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/cross-repo-dependency-analysis-gate.json}"; OUT="${2:-results/generated/cross-repo-dependency-analysis}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.cross-repo-dependency-analysis-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(((.evidence|length)>0)) ]|length) as $ok
  | {version:"patchline.cross-repo-dependency-analysis/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(((.evidence|length)>0)))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Cross-repository dependency hazard analysis"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "cross-repo-dependency-analysis worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
