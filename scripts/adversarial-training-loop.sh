#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/adversarial-training-loop-gate.json}"; OUT="${2:-results/generated/adversarial-training-loop}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.adversarial-training-loop-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.robust_after and .improved) ]|length) as $ok
  | {version:"patchline.adversarial-training-loop/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.robust_after and .improved))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Adversarial-training hardening loop"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "adversarial-training-loop worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
