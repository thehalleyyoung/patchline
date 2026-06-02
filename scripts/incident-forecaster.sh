#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/incident-forecaster-gate.json}"; OUT="${2:-results/generated/incident-forecaster}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.incident-forecaster-gate/v1"' "$SPEC" > /dev/null
jq '

  def r4: (.*10000|round)/10000;
  .forecasts as $F | .baseline_p as $b
  | (($F|map((.p-.outcome)*(.p-.outcome))|add)/($F|length)) as $brier
  | (($F|map(($b-.outcome)*($b-.outcome))|add)/($F|length)) as $bbrier
  | {version:"patchline.incident-forecaster/v1",
     n:($F|length),
     brier:($brier|r4),
     baseline_brier:($bbrier|r4),
     beats_baseline:($brier < $bbrier)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Incident-risk forecaster"; echo; echo "Brier $(jq -r .brier "$OUT/out.json") vs baseline $(jq -r .baseline_brier "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "incident-forecaster worker: brier=$(jq -r .brier "$OUT/out.json") beats_baseline=$(jq -r .beats_baseline "$OUT/out.json")"
