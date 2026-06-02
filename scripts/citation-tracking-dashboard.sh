#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/citation-tracking-dashboard-gate.json}"; OUT="${2:-results/generated/citation-tracking-dashboard}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.citation-tracking-dashboard-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.doi_linked) ]|length) as $ok
  | {version:"patchline.citation-tracking-dashboard/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.doi_linked))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Citation-tracking dashboard linked to DOI"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "citation-tracking-dashboard worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
