#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/dossier-gate.json}"
OUT="${2:-results/generated/dossier-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.dossier-gate/v1" and (.claim|length) > 200 and (.capabilities|length) >= 1' "$SPEC" > /dev/null

for phrase in "release-readiness dossier" "evidence chain" "make dossier-gate"; do
  grep -F "$phrase" docs/dossier.md README.md > /dev/null
done

bash scripts/dossier.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in dossier.json dossier.md README.md; do
  test -s "$OUT/$output"
done

# Every sampled real capability is fully certified across all six artifacts; the phantom
# capability is uncertified.
jq -e '
  .version == "patchline.dossier/v1" and
  .all_certified == true and
  .certified_count == .total and
  (.total >= 9) and
  (.uncertified | length) == 0 and
  (.capabilities | all(.[]; .example and .worker and .gate and .doc and .make and .readme)) and
  .phantom.certified == false
' "$OUT/dossier.json" > /dev/null

jq -n --slurpfile r "$OUT/dossier.json" '{
  version: "patchline.dossier-gate-results/v1",
  certified_count: $r[0].certified_count,
  total: $r[0].total,
  phantom_uncertified: ($r[0].phantom.certified | not),
  verified: true
}' > "$OUT/gate-summary.json"

echo "dossier gate passed: all sampled capabilities certified across six artifacts, phantom rejected"
