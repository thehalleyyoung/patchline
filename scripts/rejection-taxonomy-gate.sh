#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/rejection-taxonomy-gate.json}"
OUT="${2:-results/generated/rejection-taxonomy-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.rejection-taxonomy-gate/v1" and (.categories|length)==4' "$SPEC" > /dev/null

for phrase in "rejection taxonomy" "unsafe SQL" "broad writes" "missing rollback" "unbounded runtime" "make rejection-taxonomy-gate"; do
  grep -F "$phrase" docs/rejection-taxonomy.md README.md > /dev/null
done

bash scripts/rejection-taxonomy.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in classifications.jsonl rejection-taxonomy.json rejection-taxonomy.md README.md; do
  test -s "$OUT/$output"
done

# Categories declared in the spec must exactly match the categories the classifier reports.
spec_cats="$(jq -c '.categories | sort' "$SPEC")"
out_cats="$(jq -c '.by_category | keys | sort' "$OUT/rejection-taxonomy.json")"
if [ "$spec_cats" != "$out_cats" ]; then echo "category mismatch: $spec_cats vs $out_cats"; exit 1; fi

minc="$(jq '.minimum_candidates' "$SPEC")"

jq -e --argjson minc "$minc" '
  .version == "patchline.rejection-taxonomy/v1" and
  .candidates >= $minc and
  .every_category_fires == true and
  .stable == true and
  .negative_control_clean == true and
  .negative_control_safe_codes == 0 and
  (.rejected + .accepted == .candidates)
' "$OUT/rejection-taxonomy.json" > /dev/null

# Independently re-verify: every rejected candidate must carry >=1 code from the closed set,
# and every accepted candidate must carry zero codes (no silent / out-of-taxonomy rejection).
allowed='["unsafe-sql","broad-write","missing-rollback","unbounded-runtime"]'
bad="$(jq -s --argjson allowed "$allowed" '
  map(select(
    ((.rejected) and ((.rejections|length)==0)) or
    ((.rejected|not) and ((.rejections|length)>0)) or
    (.rejections | any(. as $c | ($allowed | index($c)) | not))
  )) | length
' "$OUT/classifications.jsonl")"
if [ "$bad" -ne 0 ]; then echo "found $bad inconsistent classifications"; exit 1; fi

jq -n --slurpfile r "$OUT/rejection-taxonomy.json" '{
  version: "patchline.rejection-taxonomy-gate-results/v1",
  candidates: $r[0].candidates,
  rejected: $r[0].rejected,
  by_category: $r[0].by_category,
  every_category_fires: $r[0].every_category_fires,
  negative_control_clean: $r[0].negative_control_clean,
  verified: true
}' > "$OUT/gate-summary.json"

echo "rejection taxonomy gate passed: $(jq '.rejected' "$OUT/gate-summary.json")/$(jq '.candidates' "$OUT/gate-summary.json") rejected, all 4 categories fire, negative control clean"
