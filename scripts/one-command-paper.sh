#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/one-command-paper-gate.json}"; OUT="${2:-results/generated/one-command-paper}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.one-command-paper-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.regenerated) ]|length) as $ok
  | {version:"patchline.one-command-paper/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.regenerated))}
' "$SPEC" > "$OUT/out.json"
{ echo "# One-command reproduction of the entire paper"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "one-command-paper worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
