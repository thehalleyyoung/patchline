#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/rollback-check-gate.json}"
OUT="${2:-results/generated/rollback-check-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.rollback-check-gate/v1" and (.claim|length) > 200 and (.operations|length) >= 1' "$SPEC" > /dev/null

for phrase in "rollback" "irreversible" "make rollback-check-gate"; do
  grep -F "$phrase" docs/rollback-check.md README.md > /dev/null
done

bash scripts/rollback-check.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in rollback-check.json rollback-check.md README.md; do
  test -s "$OUT/$output"
done

cls() { jq -r --arg id "$1" '.results[] | select(.id==$id) | .class' "$OUT/rollback-check.json"; }

# Each operation classified per its rollback semantics.
jq -e '
  .version == "patchline.rollback-check/v1" and
  ([.results[] | select(.id=="m1")][0].class == "reversible") and
  ([.results[] | select(.id=="m2")][0].class == "data_lossy") and
  ([.results[] | select(.id=="m3")][0].class == "irreversible") and
  ([.results[] | select(.id=="m4")][0].class == "data_lossy") and
  ([.results[] | select(.id=="m5")][0].class == "partial")
' "$OUT/rollback-check.json" > /dev/null

jq -n --slurpfile r "$OUT/rollback-check.json" '{
  version: "patchline.rollback-check-gate-results/v1",
  results: [$r[0].results[] | {id, class}],
  verified: true
}' > "$OUT/gate-summary.json"

echo "rollback-check gate passed: reversible/data_lossy/irreversible/partial all classified correctly"
