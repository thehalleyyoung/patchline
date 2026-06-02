#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/triage-prioritizer-gate.json}"; OUT="${2:-results/generated/triage-prioritizer}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.triage-prioritizer-gate/v1"' "$SPEC" > /dev/null
jq '

  def r4: (.*10000|round)/10000;
  .findings as $F
  | ([ $F | group_by(.root)[] | .[0] ]) as $dedup
  | ([ $dedup[] | . + {score:((.severity*.confidence)|r4)} ] | sort_by(-.score)) as $ranked
  | {version:"patchline.triage-prioritizer/v1",
     input:($F|length), deduped:($dedup|length),
     duplicates_removed:(($F|length)-($dedup|length)),
     top:$ranked[0].root,
     ordered:([ range(1;($ranked|length)) as $i | ($ranked[$i-1].score >= $ranked[$i].score) ]|all)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Triage prioritizer"; echo; echo "Input $(jq -r .input "$OUT/out.json") -> deduped $(jq -r .deduped "$OUT/out.json"); top $(jq -r .top "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "triage-prioritizer worker: deduped=$(jq -r .deduped "$OUT/out.json") ordered=$(jq -r .ordered "$OUT/out.json")"
