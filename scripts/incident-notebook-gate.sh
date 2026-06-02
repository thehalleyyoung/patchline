#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/incident-notebook-gate.json}"
OUT="${2:-results/generated/incident-notebook-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.incident-notebook-gate/v1" and (.required_cells | length) >= 5' "$SPEC" > /dev/null

for phrase in "Replayable incident notebook" "hypothesis" "Cells" "replay" "make incident-notebook-gate"; do
  grep -F "$phrase" docs/incident-notebook.md README.md > /dev/null
done

bash scripts/incident-notebook.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in incident-notebook.json incident-notebook.replay.json incident-notebook-result.json incident-notebook.md runtime-evidence.jsonl README.md; do
  test -s "$OUT/$output"
done

required_cells="$(jq -c '.required_cells' "$SPEC")"

# The notebook must contain exactly the required cells in order, each with an output.
jq -e --argjson required "$required_cells" '
  (.cells | map(.id)) == $required and
  all(.cells[]; has("output") and has("source"))
' "$OUT/incident-notebook.json" > /dev/null

jq -e '
  .version == "patchline.incident-notebook-result/v1" and
  .incident_is_real_finding == true and
  .replay_identical == true and
  (.hypothesis_supported | type == "boolean") and
  (.cell_ids | length) >= 6
' "$OUT/incident-notebook-result.json" > /dev/null

# Replay must be byte-identical (independent verification).
cmp -s "$OUT/incident-notebook.json" "$OUT/incident-notebook.replay.json"

jq -n --slurpfile r "$OUT/incident-notebook-result.json" '{
  version: "patchline.incident-notebook-gate-results/v1",
  cells: $r[0].cells,
  incident_id: $r[0].incident_id,
  hypothesis_supported: $r[0].hypothesis_supported,
  replay_identical: $r[0].replay_identical,
  verified: true
}' > "$OUT/gate-summary.json"

echo "incident notebook gate passed: cells $(jq '.cells' "$OUT/gate-summary.json"), incident $(jq -r '.incident_id' "$OUT/gate-summary.json"), supported $(jq '.hypothesis_supported' "$OUT/gate-summary.json"), replay $(jq '.replay_identical' "$OUT/gate-summary.json")"
