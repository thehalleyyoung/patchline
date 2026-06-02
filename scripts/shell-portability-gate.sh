#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/shell-portability-gate.json}"
OUT="${2:-results/generated/shell-portability-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.shell-portability-gate/v1" and (.claim|length) > 200 and (.clean_scripts|length) >= 1' "$SPEC" > /dev/null

for phrase in "portability" "negative-control" "make shell-portability-gate"; do
  grep -F "$phrase" docs/shell-portability.md README.md > /dev/null
done

bash scripts/shell-portability.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in shell-portability.json shell-portability.md README.md; do
  test -s "$OUT/$output"
done

# Every shipped script is clean; the negative control is flagged for all three
# catalogued hazards (mapfile, tmp_write, gnu_sed_no_suffix).
jq -e '
  .version == "patchline.shell-portability/v1" and
  .all_clean == true and
  .negative_control.flagged == true and
  (.negative_control.hazards | index("mapfile") != null) and
  (.negative_control.hazards | index("tmp_write") != null) and
  (.negative_control.hazards | index("gnu_sed_no_suffix") != null)
' "$OUT/shell-portability.json" > /dev/null

jq -n --slurpfile r "$OUT/shell-portability.json" '{
  version: "patchline.shell-portability-gate-results/v1",
  all_clean: $r[0].all_clean,
  negative_control_hazards: $r[0].negative_control.hazards,
  verified: true
}' > "$OUT/gate-summary.json"

echo "shell-portability gate passed: shipped scripts clean, negative control flagged for all hazards"
