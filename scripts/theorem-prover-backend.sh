#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/theorem-prover-backend-gate.json}"; OUT="${2:-results/generated/theorem-prover-backend}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.theorem-prover-backend-gate/v1"' "$SPEC" > /dev/null
jq '

  def r4: (.*10000|round)/10000;
  .obligations as $O | .unprovable as $U
  | ([ $O[] | select(.status=="proved" and ((.proof|length)>0)) ]|length) as $ok
  | {version:"patchline.theorem-prover-backend/v1",
     obligations:($O|length), proved:$ok,
     proved_rate:(($ok/($O|length))|r4),
     all_proved:($ok==($O|length)),
     unprovable_proved:($U.status=="proved")}

' "$SPEC" > "$OUT/out.json"
{ echo "# Theorem-prover backend"; echo; echo "Proved $(jq -r .proved "$OUT/out.json")/$(jq -r .obligations "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "theorem-prover-backend worker: all_proved=$(jq -r .all_proved "$OUT/out.json")"
