#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/parser-dashboard-gate.json}"
OUT="${2:-results/generated/parser-dashboard-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.parser-dashboard-gate/v1"' "$SPEC" > /dev/null

for phrase in "coverage" "fuzz" "known gaps" "real-repo" "make parser-dashboard-gate"; do
  grep -F "$phrase" docs/parser-dashboard.md README.md > /dev/null
done

bash scripts/parser-dashboard.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in parser-dashboard.json parser-dashboard.md README.md; do
  test -s "$OUT/$output"
done

min_eco="$(jq '.minimum_ecosystems' "$SPEC")"
min_fuzz="$(jq '.minimum_fuzz_seeds' "$SPEC")"

jq -e \
  --argjson min_eco "$min_eco" \
  --argjson min_fuzz "$min_fuzz" '
  .version == "patchline.parser-dashboard/v1" and
  .ecosystems >= $min_eco and
  .all_ecosystems_have_real_proof == true and
  .known_gaps >= 1 and
  .fuzz_seeds >= $min_fuzz and
  .fuzz_no_crashes == true and
  .fuzz_survived == .fuzz_seeds
' "$OUT/parser-dashboard.json" > /dev/null

jq -n --slurpfile r "$OUT/parser-dashboard.json" '{
  version: "patchline.parser-dashboard-gate-results/v1",
  ecosystems: $r[0].ecosystems,
  real_repo_proofs: $r[0].real_repo_proofs,
  known_gaps: $r[0].known_gaps,
  fuzz_seeds: $r[0].fuzz_seeds,
  verified: true
}' > "$OUT/gate-summary.json"

echo "parser-dashboard gate passed: every ecosystem has a real-repo proof, all fuzz seeds survived"
