#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/llm-judge-harness-gate.json}"; OUT="${2:-results/generated/llm-judge-harness}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.llm-judge-harness-gate/v1"' "$SPEC" > /dev/null
jq '

  def r4: (.*10000|round)/10000;
  .items as $I | .min_agreement as $min | .unreliable as $U
  | ([ $I[] | select(.judge_a==.judge_b) ]|length) as $agree
  | ([ $U[] | select(.judge_a==.judge_b) ]|length) as $uagree
  | {version:"patchline.llm-judge-harness/v1",
     items:($I|length),
     agreement:(($agree/($I|length))|r4),
     reliable:((($agree/($I|length))) >= $min),
     unreliable_reliable:((($uagree/($U|length))) >= $min)}

' "$SPEC" > "$OUT/out.json"
{ echo "# LLM-judge harness"; echo; echo "Agreement $(jq -r .agreement "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "llm-judge-harness worker: agreement=$(jq -r .agreement "$OUT/out.json") reliable=$(jq -r .reliable "$OUT/out.json")"
