#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/quarterly-benchmark-report-gate.json}"; OUT="${2:-results/generated/quarterly-benchmark-report}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.quarterly-benchmark-report-gate/v1"' "$SPEC" > /dev/null
jq '

  .quarters as $Q | .regression_quarter as $R
  | ([ range(1;($Q|length)) as $i | ($Q[$i].q > $Q[$i-1].q) ]|all) as $ordered
  | ([ range(1;($Q|length)) as $i | ($Q[$i].recall >= $Q[$i-1].recall) ]|all) as $nonreg
  | {version:"patchline.quarterly-benchmark-report/v1",
     quarters:($Q|length), ordered:$ordered, non_regressing:$nonreg,
     latest_recall:$Q[-1].recall,
     regression_nonreg:($R.recall >= $Q[-1].recall)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Quarterly benchmark report"; echo; echo "Quarters $(jq -r .quarters "$OUT/out.json"); latest recall $(jq -r .latest_recall "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "quarterly-benchmark-report worker: non_regressing=$(jq -r .non_regressing "$OUT/out.json")"
