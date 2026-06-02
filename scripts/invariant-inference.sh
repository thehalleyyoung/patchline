#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/invariant-inference-gate.json}"; OUT="${2:-results/generated/invariant-inference}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.invariant-inference-gate/v1"' "$SPEC" > /dev/null
jq '

  .candidate_invariants as $C
  | ([ $C[] | select(.holds) ]) as $survivors
  | ([ $C[] | select(.holds|not) ]|length) as $discarded
  | {version:"patchline.invariant-inference/v1",
     candidates:($C|length),
     survivors:($survivors|length),
     discarded:$discarded,
     all_have_obligation:($survivors|length>0),
     counterexample_discarded:($discarded>0)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Automatic invariant inference"; echo; echo "Survivors $(jq -r .survivors "$OUT/out.json"); discarded $(jq -r .discarded "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "invariant-inference worker: survivors=$(jq -r .survivors "$OUT/out.json")"
