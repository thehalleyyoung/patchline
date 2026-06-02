#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/rebuttal-evidence-pack-gate.json}"; OUT="${2:-results/generated/rebuttal-evidence-pack}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.rebuttal-evidence-pack-gate/v1"' "$SPEC" > /dev/null
jq '

  .questions as $Q | .unanswered_question as $U
  | ([ $Q[] | select((.command|length>0) and (.expected|length>0)) ]|length) as $ok
  | {version:"patchline.rebuttal-evidence-pack/v1",
     questions:($Q|length), answered:$ok,
     all_answered:($ok==($Q|length)),
     unanswered_ok:(($U.command|length)>0)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Rebuttal evidence pack"; echo; echo "Answered $(jq -r .answered "$OUT/out.json")/$(jq -r .questions "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "rebuttal-evidence-pack worker: all_answered=$(jq -r .all_answered "$OUT/out.json")"
