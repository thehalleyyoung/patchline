#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/good-first-issue-gen-gate.json}"; OUT="${2:-results/generated/good-first-issue-gen}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.good-first-issue-gen-gate/v1"' "$SPEC" > /dev/null
jq '

  .catalog_gaps as $G | .issues as $I | .fabricated_issue as $F
  | ([ $G[].gap ]) as $gaps
  | ([ $I[] | select((.scope|length>0) and (.backing_gap as $b | ($gaps|index($b))!=null)) ]|length) as $ok
  | {version:"patchline.good-first-issue-gen/v1",
     gaps:($G|length), issues:($I|length), actionable:$ok,
     all_backed:($ok==($I|length)),
     fabricated_backed:(($gaps|index($F.backing_gap))!=null)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Good-first-issue generator"; echo; echo "Issues $(jq -r .issues "$OUT/out.json"); actionable $(jq -r .actionable "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "good-first-issue-gen worker: all_backed=$(jq -r .all_backed "$OUT/out.json")"
