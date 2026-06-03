#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/maintainer-acceptance-study.json}"
OUT="${2:-results/generated/maintainer-acceptance-study-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.maintainer-acceptance-study/v1" and
  .criteria.min_pairs == 9 and
  (.tasks | length) == 3 and
  (.observations | length) == 18 and
  ([.tasks[].artifact_paths[]] | length) == 6
' "$SPEC" > /dev/null

for path in $(jq -r '.tasks[].artifact_paths[]' "$SPEC"); do
  test -s "$path"
done

for phrase in "Maintainer acceptance study" "maintainer-acceptance-study" "make maintainer-acceptance-study-gate"; do
  grep -F "$phrase" docs/maintainer-acceptance-study.md README.md > /dev/null
done

go test ./internal/acceptancestudy -run 'TestBuildReport|TestReadSpec'
go test ./cmd/patchline -run TestMaintainerAcceptanceStudyCommandWritesReports

go run ./cmd/patchline maintainer-acceptance-study \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/maintainer-acceptance-study.json"
test -s "$OUT/safe/maintainer-acceptance-study.md"

jq -e '
  .version == "patchline.maintainer-acceptance-study-report/v1" and
  .ok == true and
  .summary.pairs == 9 and
  .summary.participants == 3 and
  .summary.review_time_reduction_pct >= 35 and
  .summary.generated_uncertainty_recall == 1 and
  .summary.counterexamples == 0 and
  ([.tasks[].artifacts[]] | length) == 6 and
  ([.tasks[].artifacts[] | select(.sha256 | length == 64)] | length) == 6
' "$OUT/safe/maintainer-acceptance-study.json" > /dev/null

jq '
  (.observations[] | select(.task_id == "patch-series-review" and .condition == "generated_plan") | .uncertainty_items_identified) = ["intermediate-state-invariants"] |
  (.observations[] | select(.task_id == "patch-series-review" and .condition == "generated_plan") | .generated_plan_uncertainties) = ["intermediate-state-invariants"] |
  (.observations[] | select(.task_id == "patch-series-review" and .condition == "generated_plan") | .confidence) = 0.95
' "$SPEC" > "$OUT/hidden-uncertainty-spec.json"

go run ./cmd/patchline maintainer-acceptance-study \
  --spec "$OUT/hidden-uncertainty-spec.json" \
  --root . \
  --out "$OUT/hidden" \
  --json > "$OUT/hidden.stdout.json"

jq -e '
  .ok == false and
  .summary.counterexamples >= 3 and
  (.counterexamples | any(.kind == "hidden_uncertainty")) and
  (.counterexamples | any(.kind == "overconfidence")) and
  ([.pairs[] | select(.task_id == "patch-series-review" and (.generated_missing_uncertainties | index("rollback-gap")))] | length) == 3
' "$OUT/hidden/maintainer-acceptance-study.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/maintainer-acceptance-study.json")"
go run ./cmd/patchline maintainer-acceptance-study \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/maintainer-acceptance-study.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: maintainer acceptance study hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/maintainer-acceptance-study.json" --slurpfile hidden "$OUT/hidden/maintainer-acceptance-study.json" '{
  version: "patchline.maintainer-acceptance-study-gate-results/v1",
  pairs: $safe[0].summary.pairs,
  review_time_reduction_pct: $safe[0].summary.review_time_reduction_pct,
  generated_uncertainty_recall: $safe[0].summary.generated_uncertainty_recall,
  hidden_counterexamples: $hidden[0].counterexamples,
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "maintainer acceptance study gate passed: generated plans reduce paired review time without hiding uncertainty"
