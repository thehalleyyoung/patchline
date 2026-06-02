#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/privacy-metrics-gate.json}"
OUT="${2:-results/generated/privacy-metrics-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.privacy-metrics-gate/v1" and
  (.claim | length) > 100 and
  (.real_code | length) >= 2 and
  (.required_privacy_fields | length) >= 4 and
  (.forbidden_terms | length) >= 4
' "$SPEC" > /dev/null

for field in source_free raw_evidence_free path_free salted_cohort_ids; do
  grep -F "$field" docs/privacy-aggregate-metrics.md > /dev/null
done
grep -F "make privacy-metrics-gate" README.md > /dev/null

go test ./cmd/patchline -run TestRepoMetricsWritesPrivacyPreservingAggregates > "$OUT/go-test.log"

analysis_dirs=()
count="$(jq '.real_code | length' "$SPEC")"
for ((i=0; i<count; i++)); do
  repo="$(jq -r ".real_code[$i].repo" "$SPEC")"
  ref="$(jq -r ".real_code[$i].ref" "$SPEC")"
  subpath="$(jq -r ".real_code[$i].subpath" "$SPEC")"
  analysis="$OUT/analysis-$i"
  analysis_dirs+=("$analysis")
  go run ./cmd/patchline repo analyze \
    --github "$repo" \
    --ref "$ref" \
    --subpath "$subpath" \
    --download-dir "$OUT/cache" \
    --stages inventory,baseline,propose,compare \
    --proposal-kind all \
    --budget files=3,lines=70,tokens=10000,changes=2 \
    --no-llm \
    --out "$analysis" \
    --json > "$OUT/analyze-$i.json"
done

IFS=,
analyses="${analysis_dirs[*]}"
unset IFS

go run ./cmd/patchline repo metrics \
  --analyses "$analyses" \
  --salt "privacy-metrics-gate-salt" \
  --out "$OUT/metrics" \
  --json > "$OUT/metrics-stdout.json"

jq -e --argjson expected "$count" '
  .version == "patchline.repo-metrics/v1" and
  .shareable == true and
  .privacy.source_free == true and
  .privacy.raw_evidence_free == true and
  .privacy.path_free == true and
  .privacy.salted_cohort_ids == true and
  (.analyses | length) == $expected and
  (.trend_deltas | length) == ($expected - 1) and
  (.summary.total_ranked_risks > 0) and
  (.redacted_keys | index("path")) and
  (.redacted_keys | index("prompt")) and
  (.redacted_keys | index("generated"))
' "$OUT/metrics/metrics.json" > /dev/null

while IFS= read -r term; do
  if grep -F "$term" "$OUT/metrics/metrics.json" "$OUT/metrics/metrics.md" > /dev/null; then
    echo "privacy metrics leaked forbidden term: $term" >&2
    exit 1
  fi
done < <(jq -r '.forbidden_terms[]' "$SPEC")

for dir in "${analysis_dirs[@]}"; do
  if grep -F "$dir" "$OUT/metrics/metrics.json" "$OUT/metrics/metrics.md" > /dev/null; then
    echo "privacy metrics leaked local analysis path: $dir" >&2
    exit 1
  fi
done

grep -F "privacy-preserving aggregate metrics" "$OUT/metrics/metrics.md" > /dev/null
grep -F "Suppressed fields" "$OUT/metrics/metrics.md" > /dev/null

jq -n \
  --slurpfile metrics "$OUT/metrics/metrics.json" \
  '{
    version:"patchline.privacy-metrics-gate-results/v1",
    analyses:($metrics[0].summary.analyses),
    ranked_risks:($metrics[0].summary.total_ranked_risks),
    trend_deltas:($metrics[0].trend_deltas | length),
    source_free:$metrics[0].privacy.source_free,
    raw_evidence_free:$metrics[0].privacy.raw_evidence_free,
    path_free:$metrics[0].privacy.path_free,
    verified:true
  }' > "$OUT/summary.json"

jq -e '.verified == true and .analyses >= 2 and .ranked_risks > 0 and .source_free == true and .raw_evidence_free == true and .path_free == true' "$OUT/summary.json" > /dev/null

echo "privacy metrics gate passed: analyses $(jq '.analyses' "$OUT/summary.json"), ranked risks $(jq '.ranked_risks' "$OUT/summary.json")"
