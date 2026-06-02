#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/results-regeneration-gate.json}"; OUT="${2:-results/generated/results-regeneration}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.results-regeneration-gate/v1"' "$SPEC" > /dev/null
jq '

  def r4: (.*10000|round)/10000;
  .artifacts as $A | .nondeterministic_artifact as $N
  | ([ $A[] | select(.run_a==.run_b) ]|length) as $ok
  | {version:"patchline.results-regeneration/v1",
     artifacts:($A|length), deterministic:$ok,
     determinism_rate:(($ok/($A|length))|r4),
     all_deterministic:($ok==($A|length)),
     nondeterministic_matches:($N.run_a==$N.run_b)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Deterministic results regeneration"; echo; echo "Deterministic $(jq -r .deterministic "$OUT/out.json")/$(jq -r .artifacts "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "results-regeneration worker: all_deterministic=$(jq -r .all_deterministic "$OUT/out.json")"
