#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/self-improving-loop-gate.json}"; OUT="${2:-results/generated/self-improving-loop}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.self-improving-loop-gate/v1"' "$SPEC" > /dev/null
jq '

  .failures as $F | .unbacked_proposal as $U
  | ([ $F[] | select(.explained|not) ]) as $unexp
  | ([ $unexp[] | select((.proposed_gate|length)>0) ]|length) as $proposed
  | {version:"patchline.self-improving-loop/v1",
     unexplained:($unexp|length),
     proposals:$proposed,
     all_motivated:($proposed==($unexp|length)),
     unbacked_ok:(($U.backing_failure|length)>0)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Self-improving gate-mining loop"; echo; echo "Unexplained $(jq -r .unexplained "$OUT/out.json"); proposals $(jq -r .proposals "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "self-improving-loop worker: all_motivated=$(jq -r .all_motivated "$OUT/out.json")"
