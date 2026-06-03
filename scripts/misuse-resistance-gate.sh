#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/misuse-resistance.json}"
OUT="${2:-results/generated/misuse-resistance-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.misuse-resistance/v1" and
  .as_of_date == "2026-03-01T00:00:00Z" and
  (.criteria.required_surfaces | sort) == (["adoption_metrics","certificates","scoreboards"] | sort) and
  .criteria.min_independent_reviewers == 2 and
  .criteria.min_controls_per_scenario == 3 and
  .criteria.min_control_types_per_scenario == 3 and
  .criteria.max_risk_score == 0.8 and
  .criteria.require_evidence_paths == true and
  .criteria.require_simulation == true and
  .criteria.require_public_failure_mode == true and
  .criteria.require_control_owner == true and
  .criteria.require_passed_simulation == true and
  (.scenarios | length) == 3
' "$SPEC" > /dev/null

for path in $(jq -r '.scenarios[].controls[].evidence_paths[]?' "$SPEC" | sort -u); do
  test -s "$path"
done

for phrase in "Misuse-resistance analysis" "misuse-resistance" "make misuse-resistance-gate"; do
  grep -F "$phrase" docs/misuse-resistance.md README.md > /dev/null
done

go test ./internal/misuseresistance -run 'TestBuildReport|TestReadSpec'
go test ./cmd/patchline -run TestMisuseResistanceCommandWritesReports

go run ./cmd/patchline misuse-resistance \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/misuse-resistance.json"
test -s "$OUT/safe/misuse-resistance.md"

jq -e '
  .version == "patchline.misuse-resistance-report/v1" and
  .ok == true and
  .summary.surfaces == 3 and
  .summary.scenarios == 3 and
  .summary.controls == 9 and
  .summary.failed_simulations == 0 and
  .summary.counterexamples == 0 and
  ([.surfaces[] | select(.surface == "certificates" and (.evidence | length >= 3))] | length) == 1 and
  ([.surfaces[].evidence[] | select(.sha256 | length == 64)] | length) >= 8
' "$OUT/safe/misuse-resistance.json" > /dev/null

jq '
  (.scenarios[] | select(.scenario_id == "certificate-proof-stuffing") | .reviewer_roles) = ["certificate maintainer"] |
  (.scenarios[] | select(.scenario_id == "certificate-proof-stuffing") | .controls) |= .[0:1]
' "$SPEC" > "$OUT/weak-certificate.json"
go run ./cmd/patchline misuse-resistance \
  --spec "$OUT/weak-certificate.json" \
  --root . \
  --out "$OUT/weak-certificate" \
  --json > "$OUT/weak-certificate.stdout.json"
jq -e '
  .ok == false and
  any(.counterexamples[]; .kind == "insufficient_independent_reviewers" and .subject == "certificate-proof-stuffing") and
  any(.counterexamples[]; .kind == "insufficient_controls" and .subject == "certificate-proof-stuffing") and
  any(.counterexamples[]; .kind == "insufficient_control_types" and .subject == "certificate-proof-stuffing")
' "$OUT/weak-certificate/misuse-resistance.json" > /dev/null

jq '
  (.scenarios[] | select(.scenario_id == "scoreboard-sybil-submission") | .simulations[0].passed) = false |
  (.scenarios[] | select(.scenario_id == "scoreboard-sybil-submission") | .simulations[0].observed_outcome) = "forged submission reached public scoreboard"
' "$SPEC" > "$OUT/failed-scoreboard-simulation.json"
go run ./cmd/patchline misuse-resistance \
  --spec "$OUT/failed-scoreboard-simulation.json" \
  --root . \
  --out "$OUT/failed-scoreboard-simulation" \
  --json > "$OUT/failed-scoreboard-simulation.stdout.json"
jq -e '
  .ok == false and
  .summary.failed_simulations == 1 and
  any(.counterexamples[]; .kind == "failed_simulation" and .subject == "scoreboard-sybil-submission")
' "$OUT/failed-scoreboard-simulation/misuse-resistance.json" > /dev/null

jq '
  (.scenarios[] | select(.scenario_id == "adoption-metric-inflation") | .public_failure_mode) = "" |
  (.scenarios[] | select(.scenario_id == "adoption-metric-inflation") | .risk_score) = 0.95 |
  (.scenarios[] | select(.scenario_id == "adoption-metric-inflation") | .last_reviewed) = "2025-01-01T00:00:00Z" |
  (.scenarios[] | select(.scenario_id == "adoption-metric-inflation") | .controls[0].evidence_paths) = ["../outside.md"]
' "$SPEC" > "$OUT/inflated-adoption.json"
go run ./cmd/patchline misuse-resistance \
  --spec "$OUT/inflated-adoption.json" \
  --root . \
  --out "$OUT/inflated-adoption" \
  --json > "$OUT/inflated-adoption.stdout.json"
jq -e '
  .ok == false and
  any(.counterexamples[]; .kind == "missing_public_failure_mode" and .subject == "adoption-metric-inflation") and
  any(.counterexamples[]; .kind == "risk_score_exceeded" and .subject == "adoption-metric-inflation") and
  any(.counterexamples[]; .kind == "stale_review" and .subject == "adoption-metric-inflation") and
  any(.counterexamples[]; .kind == "invalid_evidence_path" and .subject == "adoption-metric-inflation") and
  any(.counterexamples[]; .kind == "missing_control_evidence" and .subject == "adoption-metric-inflation")
' "$OUT/inflated-adoption/misuse-resistance.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/misuse-resistance.json")"
go run ./cmd/patchline misuse-resistance \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/misuse-resistance.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: misuse-resistance report hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/misuse-resistance.json" --slurpfile weak "$OUT/weak-certificate/misuse-resistance.json" --slurpfile failed "$OUT/failed-scoreboard-simulation/misuse-resistance.json" --slurpfile inflated "$OUT/inflated-adoption/misuse-resistance.json" '{
  version: "patchline.misuse-resistance-gate-results/v1",
  surfaces: $safe[0].summary.surfaces,
  scenarios: $safe[0].summary.scenarios,
  controls: $safe[0].summary.controls,
  deterministic_hash: $safe[0].hash,
  certificate_negative_control: [$weak[0].counterexamples[].kind],
  scoreboard_negative_control: [$failed[0].counterexamples[].kind],
  adoption_negative_control: [$inflated[0].counterexamples[].kind],
  verified: true
}' > "$OUT/gate-summary.json"

echo "misuse-resistance gate passed: certificates, scoreboards, and adoption metrics are checked against gaming with hashed evidence, simulations, and negative controls"
