#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/error-taxonomy-gate.json}"
OUT="${2:-results/generated/error-taxonomy-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.error-taxonomy-gate/v1" and (.claim|length) > 200 and (.errors|length) >= 6' "$SPEC" > /dev/null

for phrase in "error taxonomy" "retryable" "make error-taxonomy-gate"; do
  grep -F "$phrase" docs/error-taxonomy.md README.md > /dev/null
done

bash scripts/error-taxonomy.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in error-taxonomy.json error-taxonomy.md README.md; do
  test -s "$OUT/$output"
done

# All six pipeline stages covered; codes unique and well-formed; every entry has a
# retryable boolean and a non-empty remediation.
jq -e '
  .version == "patchline.error-taxonomy/v1" and
  .unique_codes == true and
  (.stages | (index("fetch") != null and index("parse") != null and index("analyze") != null
     and index("generate") != null and index("compare") != null and index("report") != null)) and
  (.errors | all(.[];
    (.code | test("^E_[A-Z_]+$")) and
    (.retryable | type == "boolean") and
    (.remediation | length) > 0))
' "$OUT/error-taxonomy.json" > /dev/null

jq -n --slurpfile r "$OUT/error-taxonomy.json" '{
  version: "patchline.error-taxonomy-gate-results/v1",
  count: $r[0].count,
  stages: $r[0].stages,
  verified: true
}' > "$OUT/gate-summary.json"

echo "error-taxonomy gate passed: six stages covered, codes unique + well-formed, every entry has remediation"
