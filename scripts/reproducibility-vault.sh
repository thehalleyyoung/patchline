#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/reproducibility-vault-gate.json}"; OUT="${2:-results/generated/reproducibility-vault}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.reproducibility-vault-gate/v1"' "$SPEC" > /dev/null
jq '

  .required_components as $R | .snapshots as $S | .incomplete_snapshot as $I
  | ([ $S[] | . as $s | (([ $R[] | . as $c | ($s.components|index($c))!=null ]|all) and $s.digest_ok) ]|all) as $complete
  | (([ $R[] | . as $c | ($I.components|index($c))!=null ]|all) and $I.digest_ok) as $icomplete
  | {version:"patchline.reproducibility-vault/v1",
     snapshots:($S|length), all_complete:$complete,
     incomplete_complete:$icomplete}

' "$SPEC" > "$OUT/out.json"
{ echo "# Long-term reproducibility vault"; echo; echo "Snapshots $(jq -r .snapshots "$OUT/out.json"); all complete $(jq -r .all_complete "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "reproducibility-vault worker: all_complete=$(jq -r .all_complete "$OUT/out.json")"
