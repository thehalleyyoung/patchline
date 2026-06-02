#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/fix-suggestion-engine-gate.json}"; OUT="${2:-results/generated/fix-suggestion-engine}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.fix-suggestion-engine-gate/v1"' "$SPEC" > /dev/null
jq '

  def r4: (.*10000|round)/10000;
  .hazards as $H | .bogus_fix as $B
  | ([ $H[] | select((.fix|length>0) and (.post_fix_verdict=="safe")) ]|length) as $ok
  | {version:"patchline.fix-suggestion-engine/v1",
     hazards:($H|length), remediated:$ok,
     coverage:(($ok/($H|length))|r4),
     all_remediated:($ok==($H|length)),
     bogus_clears:($B.post_fix_verdict=="safe")}

' "$SPEC" > "$OUT/out.json"
{ echo "# Fix-suggestion engine"; echo; echo "Hazards $(jq -r .hazards "$OUT/out.json"); remediated $(jq -r .remediated "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "fix-suggestion-engine worker: all_remediated=$(jq -r .all_remediated "$OUT/out.json")"
