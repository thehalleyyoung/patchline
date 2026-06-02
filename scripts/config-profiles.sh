#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/config-profiles-gate.json}"; OUT="${2:-results/generated/config-profiles}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.config-profiles-gate/v1"' "$SPEC" > /dev/null
jq '

  .profiles as $P
  | ($P[] | select(.name=="strict")) as $s
  | ($P[] | select(.name=="balanced")) as $b
  | ($P[] | select(.name=="lenient")) as $l
  | {version:"patchline.config-profiles/v1",
     profiles:($P|length),
     recall_ordered:(($s.recall >= $b.recall) and ($b.recall >= $l.recall)),
     precision_ordered:(($s.precision <= $b.precision) and ($b.precision <= $l.precision)),
     all_documented:([ $P[] | (.threshold!=null) ]|all),
     broken_ordered:(.broken_profile.recall >= $b.recall)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Configuration profiles"; echo; echo "Profiles $(jq -r .profiles "$OUT/out.json"); recall ordered $(jq -r .recall_ordered "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "config-profiles worker: recall_ordered=$(jq -r .recall_ordered "$OUT/out.json")"
