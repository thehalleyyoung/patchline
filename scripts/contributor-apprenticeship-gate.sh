#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/contributor-apprenticeship.json}"
OUT="${2:-results/generated/contributor-apprenticeship-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.contributor-apprenticeship/v1" and
  (.claim | length) > 250 and
  (.tracks | length) == 3 and
  (.criteria.required_deliverables | sort) == ["detector","doc","fixture","gate"] and
  .criteria.max_fixture_bytes == 8192
' "$SPEC" > /dev/null

for phrase in "Contributor apprenticeship pathway" "make contributor-apprenticeship-gate" "real detector"; do
  grep -F "$phrase" docs/contributor-apprenticeship.md README.md > /dev/null
done

go test ./internal/education -run 'TestBuildApprenticeshipReport|TestReadApprenticeshipSpec|TestWriteApprenticeshipArtifacts' -count=1
go test ./cmd/patchline -run TestContributorApprenticeshipCommandWritesReports -count=1

go run ./cmd/patchline contributor-apprenticeship \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/contributor-apprenticeship.json"
test -s "$OUT/safe/contributor-apprenticeship.md"

jq -e '
  .version == "patchline.contributor-apprenticeship-report/v1" and
  .ok == true and
  .summary.tracks == 3 and
  .summary.graduated_tracks == 3 and
  .summary.gate_backed_tracks == 3 and
  .summary.deliverables_verified == 12 and
  .summary.evidence_artifacts == 15 and
  .summary.minimized_fixtures == 3 and
  .summary.negative_controls == 3 and
  ([.tracks[].evidence[] | select(.sha256 | length == 64)] | length) == 15
' "$OUT/safe/contributor-apprenticeship.json" > /dev/null

jq '
  .tracks[0].detector.symbol = "MissingDetectorSymbol" |
  .tracks[0].gate.name = "missing-gate" |
  .tracks[0].gate.commands = [] |
  .tracks[0].gate.negative_controls = [] |
  .tracks[0].documentation.required_phrases = ["phrase absent from the doc"] |
  .tracks[0].fixture.minimized = false |
  .tracks[0].fixture.negative_control_path = "examples/missing-negative-control.json" |
  .tracks[0].review.reviewers = ["single-reviewer"] |
  .tracks[0].review.mentor_signoff = false
' "$SPEC" > "$OUT/bad-spec.json"

go run ./cmd/patchline contributor-apprenticeship \
  --spec "$OUT/bad-spec.json" \
  --root . \
  --out "$OUT/bad" \
  --json > "$OUT/bad.stdout.json"

jq -e '
  .ok == false and
  (.counterexamples | any(.kind == "missing_detector_symbol")) and
  (.counterexamples | any(.kind == "missing_gate")) and
  (.counterexamples | any(.kind == "missing_reproducible_command")) and
  (.counterexamples | any(.kind == "missing_doc_phrase")) and
  (.counterexamples | any(.kind == "fixture_not_minimized")) and
  (.counterexamples | any(.kind == "missing_negative_control")) and
  (.counterexamples | any(.kind == "missing_negative_control_detail")) and
  (.counterexamples | any(.kind == "insufficient_reviewers")) and
  (.counterexamples | any(.kind == "mentor_signoff_missing")) and
  (.counterexamples | any(.kind == "deliverable_unverified"))
' "$OUT/bad/contributor-apprenticeship.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/contributor-apprenticeship.json")"
go run ./cmd/patchline contributor-apprenticeship \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/contributor-apprenticeship.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: contributor apprenticeship hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/contributor-apprenticeship.json" --slurpfile bad "$OUT/bad/contributor-apprenticeship.json" '{
  version: "patchline.contributor-apprenticeship-gate-results/v1",
  tracks: $safe[0].summary.tracks,
  graduated_tracks: $safe[0].summary.graduated_tracks,
  deliverables_verified: $safe[0].summary.deliverables_verified,
  evidence_artifacts: $safe[0].summary.evidence_artifacts,
  bad_counterexamples: $bad[0].counterexamples,
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "contributor apprenticeship gate passed: detector, gate, doc, minimized fixture, negative control, mentor, and reviewer evidence all verify"
