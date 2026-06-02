#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/rl-rollout-sequencing-gate.json}"; OUT="${2:-results/generated/rl-rollout-sequencing}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.rl-rollout-sequencing-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.safe and (.reward >= .baseline)) ]|length) as $ok
  | {version:"patchline.rl-rollout-sequencing/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.safe and (.reward >= .baseline)))}
' "$SPEC" > "$OUT/out.json"
{ echo "# RL policy for safe rollout sequencing"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "rl-rollout-sequencing worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
