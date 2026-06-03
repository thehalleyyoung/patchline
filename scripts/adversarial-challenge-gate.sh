#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

REGISTRY="${1:-examples/evidence-marketplace/challenge-registry.json}"
OUT="${2:-results/generated/adversarial-challenge-gate}"

mkdir -p "$OUT"

jq -e '
  .version == "patchline.evidence-marketplace/v1" and
  (.claim | length) > 150 and
  (.challenge_track.id == "patchline-adversarial-migrations-2026") and
  .challenge_track.responsible_disclosure.public_safe_artifacts_only == true and
  .challenge_track.responsible_disclosure.embargo_days >= 30 and
  .challenge_track.scoring.min_scoreboard_score > 0 and
  (.examples | length) >= 2 and
  all(.examples[];
    (.certificate.obligations | index("responsible-disclosure-cleared")) and
    (.challenge_submission.track_id == "patchline-adversarial-migrations-2026") and
    .challenge_submission.disclosure.status == "public-safe" and
    .challenge_submission.disclosure.public_release_allowed == true and
    (.challenge_submission.migration_artifact | length) > 0
  )
' "$REGISTRY" > /dev/null

for phrase in "adversarial migration challenge" "responsible-disclosure rules" "deterministic scoring" "make adversarial-challenge-gate"; do
  grep -F "$phrase" docs/evidence-marketplace.md README.md > /dev/null
done

go test ./internal/evidencemarketplace -run 'TestPublishChallengeTrack'
go test ./cmd/patchline -run 'TestEvidenceMarketplaceChallengeCommandWritesScoreboard'

go run ./cmd/patchline evidence-marketplace challenge \
  --registry "$REGISTRY" \
  --out "$OUT/published" \
  --json > "$OUT/stdout.json"

test -s "$OUT/published/challenge.json"
test -s "$OUT/published/challenge.md"
test -s "$OUT/published/index.html"

jq -e '
  .version == "patchline.adversarial-challenge-report/v1" and
  .ok == true and
  .summary.submitted >= 2 and
  .summary.accepted == .summary.submitted and
  .summary.rejected == 0 and
  .summary.scoreboard_entries == .summary.submitted and
  .summary.analyzer_proofs == .summary.submitted and
  (.scoreboard | all(
    .scoreboard_eligible == true and
    .public_safe == true and
    .disclosure_ready == true and
    .score >= 75 and
    .breakdown.analyzer_signal > 0 and
    .breakdown.responsible_disclosure > 0 and
    .migration_analysis.high_risk >= 1 and
    .migration_analysis.analyzer_matched_expected == true and
    (.migration_analysis.report_hash | startswith("sha256:")) and
    (.migration_analysis.artifact_sha256 | startswith("sha256:"))
  ))
' "$OUT/published/challenge.json" > /dev/null

if grep -Eiq 'password=|Authorization:|AWS_SECRET_ACCESS_KEY|source_code|BEGIN PRIVATE|token=' \
  "$OUT/published/challenge.json" "$OUT/published/challenge.md" "$OUT/published/index.html"; then
  echo "FAIL: adversarial challenge output contains a high-signal private marker" >&2
  exit 1
fi

jq -n --slurpfile r "$OUT/published/challenge.json" '{
  version: "patchline.adversarial-challenge-gate-results/v1",
  scoreboard_entries: $r[0].summary.scoreboard_entries,
  analyzer_proofs: $r[0].summary.analyzer_proofs,
  public_safe: $r[0].summary.public_safe,
  disclosure_ready: $r[0].summary.disclosure_ready,
  rejected: $r[0].summary.rejected,
  verified: true
}' > "$OUT/gate-summary.json"

echo "adversarial-challenge gate passed: $(jq -r .summary.scoreboard_entries "$OUT/published/challenge.json") public-safe adversarial migrations scored with analyzer-backed proofs and responsible-disclosure checks"
