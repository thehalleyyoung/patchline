#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/neuro-symbolic-verdict-gate.json}"; OUT="${2:-results/generated/neuro-symbolic-verdict}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.neuro-symbolic-verdict-gate/v1"' "$SPEC" > /dev/null
jq '

  def decide($p;$g): if $g=="hazard" then "hazard" elif $g=="safe" then "safe" elif $p>=0.5 then "hazard" else "safe" end;
  .cases as $C
  | ([ $C[] | decide(.prior;.gate) == .expected ]|all) as $ok
  | ([ $C[] | select(.gate=="hazard" or .gate=="safe") | decide(.prior;.gate)==.gate ]|all) as $constraint_wins
  | {version:"patchline.neuro-symbolic-verdict/v1",
     cases:($C|length), all_correct:$ok, constraint_overrides:$constraint_wins}

' "$SPEC" > "$OUT/out.json"
{ echo "# Neuro-symbolic verdict"; echo; echo "Cases $(jq -r .cases "$OUT/out.json"); all correct $(jq -r .all_correct "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "neuro-symbolic-verdict worker: constraint_overrides=$(jq -r .constraint_overrides "$OUT/out.json")"
