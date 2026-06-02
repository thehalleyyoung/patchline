#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/git-history-streaming-gate.json}"; OUT="${2:-results/generated/git-history-streaming}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.git-history-streaming-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.migration_analyzed) ]|length) as $ok
  | {version:"patchline.git-history-streaming/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.migration_analyzed))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Streaming analysis from git history"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "git-history-streaming worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
