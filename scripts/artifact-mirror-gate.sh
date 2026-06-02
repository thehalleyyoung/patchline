#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/artifact-mirror-gate.json}"
OUT="${2:-results/generated/artifact-mirror-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.artifact-mirror-gate/v1" and (.claim|length) > 200 and (.artifacts|length) >= 1' "$SPEC" > /dev/null

for phrase in "artifact mirror" "quarantine" "make artifact-mirror-gate"; do
  grep -F "$phrase" docs/artifact-mirror.md README.md > /dev/null
done

bash scripts/artifact-mirror.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in artifact-mirror.json artifact-mirror.md README.md; do
  test -s "$OUT/$output"
done

# Two clean artifacts mirrored with checksums; the leaked artifact quarantined and absent
# from the public mirror directory.
jq -e '
  .version == "patchline.artifact-mirror/v1" and
  .mirrored_count == 2 and
  .quarantined_count == 1 and
  (.mirrored | all(.[]; (.checksum | length) == 64)) and
  ([.mirrored[] | select(.name=="leaked.txt")] | length) == 0 and
  ([.quarantined[] | select(.name=="leaked.txt")][0].marker == "ACCESS_TOKEN")
' "$OUT/artifact-mirror.json" > /dev/null

test ! -f "$OUT/mirror/leaked.txt"
test -f "$OUT/mirror/metrics.json"

jq -n --slurpfile r "$OUT/artifact-mirror.json" '{
  version: "patchline.artifact-mirror-gate-results/v1",
  mirrored_count: $r[0].mirrored_count,
  quarantined: $r[0].quarantined,
  verified: true
}' > "$OUT/gate-summary.json"

echo "artifact-mirror gate passed: clean artifacts mirrored with checksums, leaked artifact quarantined"
