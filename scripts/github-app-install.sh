#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/github-app-install-gate.json}"; OUT="${2:-results/generated/github-app-install}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.github-app-install-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.least_privilege and .audited) ]|length) as $ok
  | {version:"patchline.github-app-install/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.least_privilege and .audited))}
' "$SPEC" > "$OUT/out.json"
{ echo "# One-click GitHub App with least-privilege scopes"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "github-app-install worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
