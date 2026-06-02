#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/rc-rehearsal-gate.json}"
OUT="${2:-results/generated/rc-rehearsal}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.rc-rehearsal-gate/v1" and (.stages|length) >= 1' "$SPEC" > /dev/null

rehearse() {
  # $1 = jq path to the stage array; emits a rehearsal result object.
  jq -c "$1" "$SPEC" | jq -c '
    . as $stages
    | (reduce range(0; ($stages|length)) as $i ({first_fail:null};
        if .first_fail == null and ($stages[$i].passed | not)
        then {first_fail: $stages[$i].name}
        else . end)) as $r
    | {
        stages: [$stages[].name],
        all_passed: ($r.first_fail == null),
        blessed: ($r.first_fail == null),
        first_failing_stage: $r.first_fail
      }'
}

REAL="$(rehearse '.stages')"
NEG="$(rehearse '.negative_control_stages')"

jq -n --argjson real "$REAL" --argjson neg "$NEG" '{
  version: "patchline.rc-rehearsal/v1",
  release_candidate: $real,
  negative_control: $neg
}' > "$OUT/rc-rehearsal.json"

{
  echo "# Release-candidate rehearsal"
  echo
  echo "Candidate blessed: $(jq -r '.release_candidate.blessed' "$OUT/rc-rehearsal.json")"
  echo
  echo "Negative control blocked at: $(jq -r '.negative_control.first_failing_stage' "$OUT/rc-rehearsal.json")"
} > "$OUT/rc-rehearsal.md"
cp "$OUT/rc-rehearsal.md" "$OUT/README.md"

echo "rc-rehearsal worker: blessed=$(jq -r '.release_candidate.blessed' "$OUT/rc-rehearsal.json") neg_block=$(jq -r '.negative_control.first_failing_stage' "$OUT/rc-rehearsal.json")"
