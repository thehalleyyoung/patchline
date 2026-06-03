#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/resilient-analysis.json}"
OUT="${2:-results/generated/resilient-analysis-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.resilient-analysis/v1" and
  (.claim | length) > 400 and
  (.tasks | length) == 3 and
  (.workers | length) == 3 and
  .criteria.require_worker_loss_recovery == true and
  .criteria.require_cache_quarantine == true and
  .criteria.require_cache_rebuild == true and
  .criteria.require_partition_recovery == true and
  .criteria.require_deterministic_leases == true and
  .criteria.require_no_duplicate_accepted_completion == true
' "$SPEC" > /dev/null

for phrase in "Resilient distributed analysis" "worker loss" "cache corruption" "partial network partitions" "make resilient-analysis-gate"; do
  grep -F "$phrase" docs/resilient-analysis.md README.md > /dev/null
done

go test ./internal/resilientanalysis -run 'TestBuildReport|TestReadSpec' -count=1
go test ./cmd/patchline -run TestResilientAnalysisCommandWritesReports -count=1

go run ./cmd/patchline resilient-analysis \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/resilient-analysis.json"
test -s "$OUT/safe/resilient-analysis.md"

jq -e '
  .version == "patchline.resilient-analysis-report/v1" and
  .ok == true and
  .summary.workers == 3 and
  .summary.tasks == 3 and
  .summary.completed_tasks == 3 and
  .summary.worker_loss_events == 1 and
  .summary.recovered_worker_loss_tasks == 1 and
  .summary.corrupt_caches == 1 and
  .summary.quarantined_caches == 1 and
  .summary.rebuilt_caches == 1 and
  .summary.recovered_partitions == 1 and
  .summary.duplicate_accepted_completions == 0 and
  ([.cache_artifacts[] | select(.corrupt == true and .quarantined == true and .rebuilt == true)] | length) == 1
' "$OUT/safe/resilient-analysis.json" > /dev/null

jq '
  (.events) as $events |
  .events = ($events | map(select(.kind != "task_reassigned" and .kind != "cache_quarantined" and .kind != "cache_rebuilt"))) |
  .events += [($events[] | select(.id == "complete-accounts") | .id = "complete-accounts-duplicate" | .tick = 8)] |
  .partitions[0].recovered_tick = 0 |
  .cache_artifacts[0].rebuilt_path = ""
' "$SPEC" > "$OUT/bad-spec.json"

set +e
go run ./cmd/patchline resilient-analysis \
  --spec "$OUT/bad-spec.json" \
  --root . \
  --out "$OUT/bad" \
  --json > "$OUT/bad.stdout.json"
bad_status=$?
set -e
if [[ "$bad_status" -eq 0 ]]; then
  echo "FAIL: bad resilient analysis spec unexpectedly exited successfully" >&2
  exit 1
fi

jq -e '
  .ok == false and
  (.counterexamples | any(.kind == "worker_lost_without_reassignment")) and
  (.counterexamples | any(.kind == "corrupt_cache_not_quarantined")) and
  (.counterexamples | any(.kind == "corrupt_cache_not_rebuilt")) and
  (.counterexamples | any(.kind == "partition_not_recovered")) and
  (.counterexamples | any(.kind == "duplicate_accepted_completion"))
' "$OUT/bad/resilient-analysis.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/resilient-analysis.json")"
go run ./cmd/patchline resilient-analysis \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/resilient-analysis.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: resilient analysis report hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/resilient-analysis.json" --slurpfile bad "$OUT/bad/resilient-analysis.json" '{
  version: "patchline.resilient-analysis-gate-results/v1",
  completed_tasks: $safe[0].summary.completed_tasks,
  recovered_worker_loss_tasks: $safe[0].summary.recovered_worker_loss_tasks,
  corrupt_caches: $safe[0].summary.corrupt_caches,
  rebuilt_caches: $safe[0].summary.rebuilt_caches,
  recovered_partitions: $safe[0].summary.recovered_partitions,
  bad_counterexamples: $bad[0].counterexamples,
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "resilient analysis gate passed: worker loss recovered, corrupt cache quarantined/rebuilt, partition progress verified, and injected regressions rejected"
