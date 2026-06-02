#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/adversarial-robustness-gate.json}"; OUT="${2:-results/generated/adversarial-robustness}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.adversarial-robustness-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.caught) ]|length) as $ok
  | {version:"patchline.adversarial-robustness/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.caught))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Robustness against an automated adversary"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "adversarial-robustness worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
