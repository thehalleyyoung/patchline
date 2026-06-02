#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/contributor-badges-gate.json}"
OUT="${2:-results/generated/contributor-badges-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.contributor-badges-gate/v1" and (.claim|length) > 200 and (.contributors|length) >= 1' "$SPEC" > /dev/null

for phrase in "gate-backed" "recognition" "make contributor-badges-gate"; do
  grep -F "$phrase" docs/contributor-badges.md README.md > /dev/null
done

bash scripts/contributor-badges.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in contributor-badges.json badges.md README.md; do
  test -s "$OUT/$output"
done

# Every badged contribution must map to a real gate script.
jq -e '
  .version == "patchline.contributor-badges/v1" and
  (.badges | length) >= 1 and
  (.badges | all(.[]; (.backed_contributions | length) == .count))
' "$OUT/contributor-badges.json" > /dev/null

for cap in $(jq -r '.badges[].backed_contributions[]' "$OUT/contributor-badges.json"); do
  test -f "scripts/${cap}-gate.sh"
done

# Tier monotonicity: a higher count must never yield a lower tier rank.
jq -e '
  (.badges | sort_by(.count)) as $b
  | [range(0; ($b|length)-1) as $i
      | ((["none","bronze","silver","gold"] | index($b[$i].tier))
         <= (["none","bronze","silver","gold"] | index($b[$i+1].tier)))]
  | all(.)
' "$OUT/contributor-badges.json" > /dev/null

# The contributor with an unbacked capability must not be credited for it.
jq -e '
  (.badges[] | select(.handle=="carol") | .backed_contributions) as $c
  | ($c | index("this-capability-has-no-gate")) == null
' "$OUT/contributor-badges.json" > /dev/null

jq -n --slurpfile r "$OUT/contributor-badges.json" '{
  version: "patchline.contributor-badges-gate-results/v1",
  contributors: $r[0].contributors,
  verified: true
}' > "$OUT/gate-summary.json"

echo "contributor-badges gate passed: every badge backed by a real gate, tiers monotonic, unbacked claims dropped"
