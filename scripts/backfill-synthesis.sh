#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/backfill-synthesis-gate.json}"; OUT="${2:-results/generated/backfill-synthesis}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.backfill-synthesis-gate/v1"' "$SPEC" > /dev/null
jq '

  {version:"patchline.backfill-synthesis/v1",
   invariant:.invariant,
   steps:(.synthesized_steps|length),
   establishes_invariant:(.post_state_null_rows==0),
   noop_establishes:(.noop_post_null_rows==0)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Backfill program synthesis"; echo; echo "Steps $(jq -r .steps "$OUT/out.json"); establishes $(jq -r .establishes_invariant "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "backfill-synthesis worker: establishes_invariant=$(jq -r .establishes_invariant "$OUT/out.json")"
