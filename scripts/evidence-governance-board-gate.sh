#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/evidence-marketplace/governance-board.json}"
OUT="${2:-results/generated/evidence-governance-board-gate}"

mkdir -p "$OUT"

jq -e '
  .version == "patchline.evidence-governance-board/v1" and
  (.claim | length) > 180 and
  .registry_path == "governance-registry.json" and
  .board.quorum == 3 and
  .board.min_independent_approvers == 2 and
  ([.decisions[].requested_status] | sort) == ["accept","deprecate","quarantine"]
' "$SPEC" > /dev/null

for phrase in "shared evidence governance board" "accepting, deprecating, or quarantining" "archive-preserving tombstones" "make evidence-governance-board-gate"; do
  grep -F "$phrase" docs/evidence-governance-board.md README.md > /dev/null
done

go test ./internal/evidencemarketplace -run 'TestEvaluateBoardReview'
go test ./cmd/patchline -run 'TestEvidenceMarketplaceGovernCommandWritesBoardReview'

go run ./cmd/patchline evidence-marketplace govern \
  --spec "$SPEC" \
  --out "$OUT/passing" \
  --json > "$OUT/passing.stdout.json"

test -s "$OUT/passing/governance-board.json"
test -s "$OUT/passing/governance-board.md"
test -s "$OUT/passing/index.html"

jq -e '
  .version == "patchline.evidence-governance-board-report/v1" and
  .ok == true and
  .summary.submitted_decisions == 3 and
  .summary.accepted == 1 and
  .summary.deprecated == 1 and
  .summary.quarantined == 1 and
  .summary.rejected == 0 and
  .summary.active_evidence == 1 and
  .summary.independent_approvals == 6 and
  .summary.preserved_archive_artifacts == 4 and
  .summary.tombstones_required == 4 and
  .active_evidence_ids == ["redacted-rails-backfill-guard"] and
  (.decisions | length) == 3 and
  (.decisions[] | select(.final_status == "deprecated") | .archive_preservation | length) == 2 and
  (.decisions[] | select(.final_status == "quarantined") | .archive_preservation | length) == 2 and
  (.preserved_archive_entries | all(
    (.mirror_path | startswith("archive/sha256/")) and
    (.checksum | startswith("sha256:")) and
    (.withdrawal_id | startswith("sha256:")) and
    .tombstone_required == true and
    .preserve_checksum_after_withdrawal == true and
    .review_required == true
  ))
' "$OUT/passing/governance-board.json" > /dev/null

REGISTRY_ABS="$ROOT/examples/evidence-marketplace/governance-registry.json"

jq --arg registry "$REGISTRY_ABS" '(.registry_path) = $registry | (.decisions[0].reviewers) = [ .decisions[0].reviewers[0] ]' "$SPEC" > "$OUT/bad-quorum.json"
set +e
go run ./cmd/patchline evidence-marketplace govern \
  --spec "$OUT/bad-quorum.json" \
  --out "$OUT/bad-quorum" \
  --json > "$OUT/bad-quorum.stdout.json" 2> "$OUT/bad-quorum.stderr"
status=$?
set -e
if [ "$status" -eq 0 ]; then
  echo "FAIL: insufficient quorum was accepted" >&2
  exit 1
fi
jq -e '.ok == false and any(.rejected[]?.reasons[]?; contains("independent approvals must meet"))' \
  "$OUT/bad-quorum/governance-board.json" > /dev/null

jq --arg registry "$REGISTRY_ABS" '(.registry_path) = $registry | (.decisions[0].reviewers[0].affiliation) = "Patchline Public Corpus"' "$SPEC" > "$OUT/bad-conflict.json"
set +e
go run ./cmd/patchline evidence-marketplace govern \
  --spec "$OUT/bad-conflict.json" \
  --out "$OUT/bad-conflict" \
  --json > "$OUT/bad-conflict.stdout.json" 2> "$OUT/bad-conflict.stderr"
status=$?
set -e
if [ "$status" -eq 0 ]; then
  echo "FAIL: conflicted approval was accepted" >&2
  exit 1
fi
jq -e '.ok == false and any(.rejected[]?.reasons[]?; contains("conflicted reviewers may not approve"))' \
  "$OUT/bad-conflict/governance-board.json" > /dev/null

jq --arg registry "$REGISTRY_ABS" '(.registry_path) = $registry | (.decisions[2].quarantine.preserve_tombstone) = false' "$SPEC" > "$OUT/bad-tombstone.json"
set +e
go run ./cmd/patchline evidence-marketplace govern \
  --spec "$OUT/bad-tombstone.json" \
  --out "$OUT/bad-tombstone" \
  --json > "$OUT/bad-tombstone.stdout.json" 2> "$OUT/bad-tombstone.stderr"
status=$?
set -e
if [ "$status" -eq 0 ]; then
  echo "FAIL: quarantine without tombstone preservation was accepted" >&2
  exit 1
fi
jq -e '.ok == false and any(.rejected[]?.reasons[]?; contains("quarantine.preserve_tombstone must be true"))' \
  "$OUT/bad-tombstone/governance-board.json" > /dev/null

jq -n \
  --slurpfile r "$OUT/passing/governance-board.json" '{
  version: "patchline.evidence-governance-board-gate-results/v1",
  accepted: $r[0].summary.accepted,
  deprecated: $r[0].summary.deprecated,
  quarantined: $r[0].summary.quarantined,
  active_evidence: $r[0].summary.active_evidence,
  preserved_archive_artifacts: $r[0].summary.preserved_archive_artifacts,
  tombstones_required: $r[0].summary.tombstones_required,
  weak_quorum_rejected: true,
  conflicted_approval_rejected: true,
  missing_tombstone_rejected: true,
  verified: true
}' > "$OUT/gate-summary.json"

echo "evidence-governance-board gate passed: 1 accepted, 1 deprecated, 1 quarantined, 4 archive artifacts preserved with tombstones; weak quorum, conflicted approval, and missing tombstone controls rejected"
