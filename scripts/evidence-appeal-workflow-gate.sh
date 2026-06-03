#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/evidence-marketplace/appeal-workflow.json}"
OUT="${2:-results/generated/evidence-appeal-workflow-gate}"

mkdir -p "$OUT"

jq -e '
  .version == "patchline.evidence-appeal-workflow/v1" and
  (.claim | length) > 180 and
  .registry_path == "governance-registry.json" and
  .board_decisions_path == "governance-board.json" and
  .board.quorum == 3 and
  .board.min_independent_approvers == 2 and
  (.appeals | length) == 3 and
  ([.appeals[].resolution.status] | sort) == ["modified","overturned","upheld"]
' "$SPEC" > /dev/null

for phrase in "evidence appeal workflow" "Preserved evidence" "Reviewer rationale" "make evidence-appeal-workflow-gate"; do
  grep -F "$phrase" docs/evidence-appeal-workflow.md > /dev/null
done

go test ./internal/evidencemarketplace -run 'TestEvaluateAppealWorkflow'
go test ./cmd/patchline -run 'TestEvidenceMarketplaceAppealCommandWritesAppealWorkflow'

go run ./cmd/patchline evidence-marketplace appeal \
  --spec "$SPEC" \
  --out "$OUT/passing" \
  --json > "$OUT/passing.stdout.json"

test -s "$OUT/passing/appeal-workflow.json"
test -s "$OUT/passing/appeal-workflow.md"
test -s "$OUT/passing/index.html"

jq -e '
  .version == "patchline.evidence-appeal-workflow-report/v1" and
  .ok == true and
  .summary.submitted_appeals == 3 and
  .summary.processed_appeals == 3 and
  .summary.upheld == 1 and
  .summary.modified == 1 and
  .summary.overturned == 1 and
  .summary.rejected == 0 and
  .summary.board_bindings == 3 and
  .summary.preserved_artifacts == 6 and
  .summary.reviewer_rationales == 9 and
  .summary.independent_reviews == 6 and
  (.appeals | all(
    (.board_decision.evidence_hash == .evidence_hash) and
    (.board_decision.certificate_subject_hash == .certificate_subject_hash) and
    (.preserved_artifacts | length) == 2 and
    (.reviewer_rationales | length) == 3
  ))
' "$OUT/passing/appeal-workflow.json" > /dev/null

REGISTRY_ABS="$ROOT/examples/evidence-marketplace/governance-registry.json"
BOARD_ABS="$ROOT/examples/evidence-marketplace/governance-board.json"

jq --arg registry "$REGISTRY_ABS" --arg board "$BOARD_ABS" \
  '(.registry_path) = $registry | (.board_decisions_path) = $board | del(.appeals[0].preserved_artifacts[0])' \
  "$SPEC" > "$OUT/bad-preservation.json"
set +e
go run ./cmd/patchline evidence-marketplace appeal \
  --spec "$OUT/bad-preservation.json" \
  --out "$OUT/bad-preservation" \
  --json > "$OUT/bad-preservation.stdout.json" 2> "$OUT/bad-preservation.stderr"
status=$?
set -e
if [ "$status" -eq 0 ]; then
  echo "FAIL: appeal with missing preserved artifact was accepted" >&2
  exit 1
fi
jq -e '.ok == false and any(.rejected[]?.reasons[]?; contains("preserved_artifacts must include every marketplace archive mirror entry"))' \
  "$OUT/bad-preservation/appeal-workflow.json" > /dev/null

jq --arg registry "$REGISTRY_ABS" --arg board "$BOARD_ABS" \
  '(.registry_path) = $registry | (.board_decisions_path) = $board | (.appeals[0].reviewer_rationales[0].reviewer.name) = "Database Reliability Guild" | (.appeals[0].reviewer_rationales[0].reviewer.affiliation) = "Database Reliability Guild"' \
  "$SPEC" > "$OUT/bad-original-approver.json"
set +e
go run ./cmd/patchline evidence-marketplace appeal \
  --spec "$OUT/bad-original-approver.json" \
  --out "$OUT/bad-original-approver" \
  --json > "$OUT/bad-original-approver.stdout.json" 2> "$OUT/bad-original-approver.stderr"
status=$?
set -e
if [ "$status" -eq 0 ]; then
  echo "FAIL: appeal reviewer reused from original board approvers was accepted" >&2
  exit 1
fi
jq -e '.ok == false and any(.rejected[]?.reasons[]?; contains("appeal reviewers must be independent of original board approvers"))' \
  "$OUT/bad-original-approver/appeal-workflow.json" > /dev/null

jq --arg registry "$REGISTRY_ABS" --arg board "$BOARD_ABS" \
  '(.registry_path) = $registry | (.board_decisions_path) = $board | (.appeals[0].resolution.rationale) = "The final appeal rationale accidentally contains token=not-public and must be rejected before publication."' \
  "$SPEC" > "$OUT/bad-private-marker.json"
set +e
go run ./cmd/patchline evidence-marketplace appeal \
  --spec "$OUT/bad-private-marker.json" \
  --out "$OUT/bad-private-marker" \
  --json > "$OUT/bad-private-marker.stdout.json" 2> "$OUT/bad-private-marker.stderr"
status=$?
set -e
if [ "$status" -eq 0 ]; then
  echo "FAIL: private marker in appeal resolution was accepted" >&2
  exit 1
fi
jq -e '.ok == false and any(.rejected[]?.reasons[]?; contains("private marker token="))' \
  "$OUT/bad-private-marker/appeal-workflow.json" > /dev/null

jq -n \
  --slurpfile r "$OUT/passing/appeal-workflow.json" '{
  version: "patchline.evidence-appeal-workflow-gate-results/v1",
  processed_appeals: $r[0].summary.processed_appeals,
  upheld: $r[0].summary.upheld,
  modified: $r[0].summary.modified,
  overturned: $r[0].summary.overturned,
  preserved_artifacts: $r[0].summary.preserved_artifacts,
  reviewer_rationales: $r[0].summary.reviewer_rationales,
  missing_preservation_rejected: true,
  original_approver_rejected: true,
  private_marker_rejected: true,
  verified: true
}' > "$OUT/gate-summary.json"

echo "evidence-appeal-workflow gate passed: 3 appeals processed with 6 preserved artifacts, 9 reviewer rationales, and rejection controls for missing preservation, original approvers, and private markers"
