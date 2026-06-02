#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/canonical-json-gate.json}"
OUT="${2:-results/generated/canonical-json-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.canonical-json-gate/v1" and (.claim|length) > 200 and (.doc_a|type=="object")' "$SPEC" > /dev/null

for phrase in "canonical" "checksum" "make canonical-json-gate"; do
  grep -F "$phrase" docs/canonical-json.md README.md > /dev/null
done

bash scripts/canonical-json.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in canonical-json.json canonical-json.md README.md; do
  test -s "$OUT/$output"
done

# Reordered-but-equal twin collides under canonical form and checksum; a
# content-changed document produces a different checksum.
jq -e '
  .version == "patchline.canonical-json/v1" and
  .reordered_collides == true and
  .content_change_differs == true and
  (.canonical.a == .canonical.b) and
  (.checksum.a == .checksum.b) and
  (.checksum.a != .checksum.c)
' "$OUT/canonical-json.json" > /dev/null

jq -n --slurpfile r "$OUT/canonical-json.json" '{
  version: "patchline.canonical-json-gate-results/v1",
  reordered_collides: $r[0].reordered_collides,
  content_change_differs: $r[0].content_change_differs,
  verified: true
}' > "$OUT/gate-summary.json"

echo "canonical-json gate passed: reordered twin collides, content change differs"
