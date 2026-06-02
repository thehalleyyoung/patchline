#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/continual-learning-eval-gate.json}"; OUT="${2:-results/generated/continual-learning-eval}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.continual-learning-eval-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.old_accuracy >= .min) ]|length) as $ok
  | {version:"patchline.continual-learning-eval/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.old_accuracy >= .min))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Continual-learning guard against forgetting"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "continual-learning-eval worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
