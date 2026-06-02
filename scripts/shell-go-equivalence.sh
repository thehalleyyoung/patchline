#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/shell-go-equivalence-gate.json}"; OUT="${2:-results/generated/shell-go-equivalence}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.shell-go-equivalence-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.shell_verdict==.go_verdict) ]|length) as $ok
  | {version:"patchline.shell-go-equivalence/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.shell_verdict==.go_verdict))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Machine-checked shell/Go gate equivalence"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "shell-go-equivalence worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
