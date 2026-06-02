#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/self-serve-onboarding-gate.json}"; OUT="${2:-results/generated/self-serve-onboarding}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.self-serve-onboarding-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select((.activation_rate>=.min) and (.retention_w4>=.min)) ]|length) as $ok
  | {version:"patchline.self-serve-onboarding/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|((.activation_rate>=.min) and (.retention_w4>=.min)))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Self-serve onboarding with activation and retention"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "self-serve-onboarding worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
