#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/repro-appendix-gate.json}"; OUT="${2:-results/generated/repro-appendix}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.repro-appendix-gate/v1"' "$SPEC" > /dev/null
jq '

  .claims as $C | .uncovered_claim as $U
  | ([ $C[] | select((.command|length>0) and (.expected|length>0)) ]|length) as $ok
  | {version:"patchline.repro-appendix/v1",
     claims:($C|length), covered:$ok,
     all_covered:($ok==($C|length)),
     uncovered_ok:(($U.command|length)>0)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Reproducibility appendix"; echo; echo "Claims $(jq -r .claims "$OUT/out.json"); covered $(jq -r .covered "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "repro-appendix worker: all_covered=$(jq -r .all_covered "$OUT/out.json")"
