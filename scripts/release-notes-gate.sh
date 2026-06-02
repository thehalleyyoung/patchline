#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/release-notes-gate.json}"
OUT="${2:-results/generated/release-notes-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.release-notes-gate/v1" and (.claim|length) > 200 and (.recognition|length) >= 1 and (.known_limitations|length) >= 1' "$SPEC" > /dev/null

for phrase in "public proof" "known-limitations" "make release-notes-gate"; do
  grep -F "$phrase" docs/release-notes.md README.md > /dev/null
done

bash scripts/release-notes.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in release-notes.json release-notes.md README.md; do
  test -s "$OUT/$output"
done

# The proof delta must be non-empty and every advertised new proof must map to a
# real gate script; recognition and limitations sections must be present.
jq -e '
  .version == "patchline.release-notes/v1" and
  .added_count >= 1 and
  .recognition_count >= 1 and
  .limitations_count >= 1 and
  (.added_gates | length) == .added_count
' "$OUT/release-notes.json" > /dev/null

for g in $(jq -r '.added_gates[]' "$OUT/release-notes.json"); do
  test -f "scripts/${g}.sh"
done

# The rendered notes must contain all three mandated sections.
for section in "## New public proofs (gate delta)" "## Contributor recognition" "## Known limitations"; do
  grep -Fq "$section" "$OUT/release-notes.md"
done

jq -n --slurpfile r "$OUT/release-notes.json" '{
  version: "patchline.release-notes-gate-results/v1",
  release: $r[0].release,
  added_count: $r[0].added_count,
  verified: true
}' > "$OUT/gate-summary.json"

echo "release-notes gate passed: $(jq -r .added_count "$OUT/release-notes.json") new proofs all gate-backed, recognition + limitations present"
