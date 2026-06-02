#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/hermetic-artifact-container-gate.json}"; OUT="${2:-results/generated/hermetic-artifact-container}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.hermetic-artifact-container-gate/v1"' "$SPEC" > /dev/null
jq '

  .checklist as $C
  | ([ $C[] | select(.satisfied) ]|length) as $ok
  | {version:"patchline.hermetic-artifact-container/v1",
     items:($C|length), satisfied:$ok,
     all_satisfied:($ok==($C|length)),
     hermetic:(.offline and .inputs_pinned and (.network_required|not)),
     leaky_hermetic:(.leaky_container.network_required|not)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Hermetic artifact-evaluation container"; echo; echo "Satisfied $(jq -r .satisfied "$OUT/out.json")/$(jq -r .items "$OUT/out.json"); hermetic $(jq -r .hermetic "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "hermetic-artifact-container worker: hermetic=$(jq -r .hermetic "$OUT/out.json")"
