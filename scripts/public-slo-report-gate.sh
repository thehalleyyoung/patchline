#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/public-slo-report.json}"
OUT="${2:-results/generated/public-slo-report-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.public-slo/v1" and
  (.claim | length) > 450 and
  (.surfaces | length) == 4 and
  (.criteria.required_kinds | length) == 4 and
  .criteria.require_public_status_url == true and
  .criteria.require_reproducibility_probe == true and
  .criteria.require_incident_review == true and
  .criteria.require_evidence_hashes == true and
  .criteria.require_reproducibility_command == true and
  ([.surfaces[].status_url] | all(test("^https://"))) and
  ([.surfaces[].probes | length] | all(. >= 3)) and
  ([.surfaces[].probes[] | select(.kind == "reproducibility").command | length] | all(. >= 2))
' "$SPEC" > /dev/null

for phrase in "Public uptime and reproducibility SLO report" "hosted docs, artifacts, marketplace evidence, and corpus APIs" "make public-slo-report-gate"; do
  grep -F "$phrase" docs/public-slo-report.md README.md > /dev/null
done

go test ./internal/sloreport -count=1
go test ./cmd/patchline -run TestPublicSLOReportCommandWritesReports -count=1

go run ./cmd/patchline public-slo-report \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/public-slo-report.json"
test -s "$OUT/safe/public-slo-report.md"

jq -e '
  .version == "patchline.public-slo-report/v1" and
  .ok == true and
  .summary.surfaces == 4 and
  .summary.kinds == 4 and
  .summary.public_status_urls == 4 and
  .summary.uptime_slo_met == 4 and
  .summary.reproducibility_slo_met == 4 and
  .summary.latency_slo_met == 4 and
  .summary.reproducibility_probes == 4 and
  .summary.reviewed_incidents == 1 and
  .summary.counterexamples == 0
' "$OUT/safe/public-slo-report.json" > /dev/null

jq '
  .surfaces |= [.[0], .[1], .[2]] |
  .surfaces[0].status_url = "" |
  .surfaces[0].probes[0].status = "fail" |
  .surfaces[0].probes[1].status = "fail" |
  .surfaces[0].probes[2].latency_ms = 3000 |
  .surfaces[0].probes[2].artifact.sha256 = "sha256:0000000000000000000000000000000000000000000000000000000000000000" |
  .surfaces[1].probes = (.surfaces[1].probes | map(select(.kind != "reproducibility"))) |
  .surfaces[1].evidence_paths = [] |
  .surfaces[2].incidents[0].corrective_action = "" |
  .surfaces[2].slo.max_incident_minutes = 5 |
  .surfaces[2].probes[0].observed_at = "2026-05-20T00:00:00Z"
' "$SPEC" > "$OUT/bad-spec.json"

set +e
go run ./cmd/patchline public-slo-report \
  --spec "$OUT/bad-spec.json" \
  --root . \
  --out "$OUT/bad" \
  --json > "$OUT/bad.stdout.json" 2> "$OUT/bad.stderr.txt"
bad_status=$?
set -e
if [[ "$bad_status" -eq 0 ]]; then
  echo "FAIL: bad public SLO report unexpectedly exited successfully" >&2
  exit 1
fi

jq -e '
  .ok == false and
  (.counterexamples | any(.kind == "missing_required_kind")) and
  (.counterexamples | any(.kind == "missing_public_status_url")) and
  (.counterexamples | any(.kind == "uptime_slo_breached")) and
  (.counterexamples | any(.kind == "reproducibility_slo_breached")) and
  (.counterexamples | any(.kind == "latency_slo_breached")) and
  (.counterexamples | any(.kind == "probe_artifact_hash_mismatch")) and
  (.counterexamples | any(.kind == "missing_reproducibility_probe")) and
  (.counterexamples | any(.kind == "missing_surface_evidence")) and
  (.counterexamples | any(.kind == "incident_review_missing")) and
  (.counterexamples | any(.kind == "incident_budget_exceeded")) and
  (.counterexamples | any(.kind == "stale_probe"))
' "$OUT/bad/public-slo-report.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/public-slo-report.json")"
go run ./cmd/patchline public-slo-report \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/public-slo-report.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: public SLO report hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/public-slo-report.json" --slurpfile bad "$OUT/bad/public-slo-report.json" '{
  version: "patchline.public-slo-report-gate-results/v1",
  surfaces: $safe[0].summary.surfaces,
  kinds: $safe[0].summary.kinds,
  probes: $safe[0].summary.probes,
  reviewed_incidents: $safe[0].summary.reviewed_incidents,
  bad_counterexamples: $bad[0].counterexamples,
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "public SLO report gate passed: hosted docs, artifacts, marketplace evidence, and corpus APIs are publicly statused, reproducible, and regression-checked"
