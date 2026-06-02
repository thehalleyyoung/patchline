#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/contributor-onboarding-gate.json}"; OUT="${2:-results/generated/contributor-onboarding}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.contributor-onboarding-gate/v1"' "$SPEC" > /dev/null
jq '

  .stages as $S | .required as $R | .incomplete as $I
  | ([ $S[].name ]) as $names
  | ([ $R[] | . as $r | ($names|index($r))!=null ]|all) as $complete
  | ([ $S[] | select((.cmd|length)>0) ]|length) as $runnable
  | ([ $I[].name ]) as $inames
  | ([ $R[] | . as $r | ($inames|index($r))!=null ]|all) as $icomplete
  | {version:"patchline.contributor-onboarding/v1",
     stages:($S|length), complete:$complete,
     all_runnable:($runnable==($S|length)),
     incomplete_complete:$icomplete}

' "$SPEC" > "$OUT/out.json"
{ echo "# Contributor onboarding"; echo; echo "Stages $(jq -r .stages "$OUT/out.json"); complete $(jq -r .complete "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "contributor-onboarding worker: complete=$(jq -r .complete "$OUT/out.json")"
