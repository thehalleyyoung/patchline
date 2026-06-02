#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/issue-to-artifact-gate.json}"
OUT="${2:-results/generated/issue-to-artifact-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.issue-to-artifact-gate/v1" and (.claim|length) > 200 and (.submissions|length) >= 2' "$SPEC" > /dev/null

for phrase in "pinned public proof" "rejected" "make issue-to-artifact-gate"; do
  grep -F "$phrase" docs/issue-to-artifact.md README.md > /dev/null
done

bash scripts/issue-to-artifact.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in issue-to-artifact.json proofs.md proofs-index.json README.md; do
  test -s "$OUT/$output"
done

# Invariants:
#  - at least one valid submission becomes a pinned proof entry;
#  - the unpinned and unknown-capability negative controls are both rejected;
#  - every accepted proof entry carries a pinned ref and a real gate script.
jq -e '
  .version == "patchline.issue-to-artifact/v1" and
  .accepted >= 1 and
  .rejected >= 2 and
  ([.results[] | select(.status=="rejected") | .reason] | (index("unpinned-ref") != null)) and
  ([.results[] | select(.status=="rejected") | .reason] | (index("unknown-capability") != null))
' "$OUT/issue-to-artifact.json" > /dev/null

# Every emitted proof entry must be reproducibly pinned and gate-backed.
jq -e 'all(.[];
  (.ref | test("^[0-9a-f]{40}$") or test("^refs/tags/")) and (.gate | startswith("scripts/")))
' "$OUT/proofs-index.json" > /dev/null

for g in $(jq -r '.[].gate' "$OUT/proofs-index.json"); do
  test -f "$g"
done

jq -n --slurpfile r "$OUT/issue-to-artifact.json" '{
  version: "patchline.issue-to-artifact-gate-results/v1",
  accepted: $r[0].accepted,
  rejected: $r[0].rejected,
  verified: true
}' > "$OUT/gate-summary.json"

echo "issue-to-artifact gate passed: $(jq -r .accepted "$OUT/issue-to-artifact.json") pinned proofs, $(jq -r .rejected "$OUT/issue-to-artifact.json") rejected incl. unpinned + unknown-capability controls"
