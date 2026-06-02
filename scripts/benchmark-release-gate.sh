#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/benchmark-release-gate.json}"; OUT="${2:-results/generated/benchmark-release}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.benchmark-release-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "frozen split" "make benchmark-release-gate"; do grep -F "$phrase" docs/benchmark-release.md README.md > /dev/null; done
bash scripts/benchmark-release.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '
  .version == "patchline.benchmark-release/v1" and
  .checksum_stable == true and .disjoint == true and .complete == true and
  .good_submission_valid == true and .bad_submission_valid == false
' "$OUT/bench.json" > /dev/null
jq -n --slurpfile r "$OUT/bench.json" '{version:"patchline.benchmark-release-gate-results/v1", checksum:$r[0].checksum, checksum_stable:$r[0].checksum_stable, submission_validated:($r[0].good_submission_valid and ($r[0].bad_submission_valid|not)), verified:true}' > "$OUT/gate-summary.json"
echo "benchmark-release gate passed: frozen split stable, disjoint/complete, submission format enforced"
