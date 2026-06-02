#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

FEEDBACK_SPEC="${1:-examples/live-feedback-ingestion-gate.json}"
SPEC="${2:-examples/safe-online-evaluation-gate.json}"
OUT="${3:-results/generated/safe-online-evaluation-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.safe-online-evaluation/v1" and (.claim | length) > 200 and (.candidate_detectors | length) >= 2' "$SPEC" > /dev/null
for phrase in "safe online-evaluation lane" "shadow_only" "make safe-online-evaluation-gate"; do
  grep -F "$phrase" docs/safe-online-evaluation.md README.md > /dev/null
done

go test ./internal/feedback -run 'TestOnlineEvaluation'
go test ./cmd/patchline -run 'TestFeedbackLiveLearningCommandsWriteReports'

go run ./cmd/patchline feedback ingest "$FEEDBACK_SPEC" --out "$OUT/ingest" --json > "$OUT/ingest.stdout.json"
go run ./cmd/patchline feedback online-eval \
  --feedback "$OUT/ingest/live-feedback.json" \
  --spec "$SPEC" \
  --out "$OUT/evaluation" \
  --json > "$OUT/evaluation.stdout.json"

test -s "$OUT/evaluation/online-evaluation.json"
test -s "$OUT/evaluation/online-evaluation.md"

jq -e '
  .version == "patchline.safe-online-evaluation-report/v1" and
  .ok == true and
  .policy_mutation_allowed == false and
  .evidence_basis == "published_k_anonymous_groups_only" and
  .summary.detectors_evaluated == 2 and
  .summary.promotion_candidates == 1 and
  .summary.shadow_only == 1 and
  (.detectors | any(.detector == "orm.write-breadth" and .status == "shadow_only" and (.gates | any(.passed == false)))) and
  (.detectors | any(.detector == "sql.destructive-ddl" and .status == "candidate_ready_for_gated_review" and .metrics.precision_bp == 10000 and .metrics.recall_bp == 10000)) and
  .privacy.source_free == true and .privacy.raw_values_free == true and .privacy.identifier_free == true
' "$OUT/evaluation/online-evaluation.json" > /dev/null

if grep -Eiq 'DROP TABLE|UPDATE accounts|diff --git|db/migrate|source_code|raw_evidence|finding_id|evidence_hash|team-alpha|local-secret-feedback-salt-2026' \
  "$OUT/evaluation/online-evaluation.json" "$OUT/evaluation/online-evaluation.md"; then
  echo "FAIL: online evaluation retained source, raw evidence, identifiers, or salt" >&2
  exit 1
fi

jq -n --slurpfile r "$OUT/evaluation/online-evaluation.json" '{
  version: "patchline.safe-online-evaluation-gate-results/v1",
  promotion_candidates: $r[0].summary.promotion_candidates,
  shadow_only: $r[0].summary.shadow_only,
  verified: true
}' > "$OUT/gate-summary.json"

echo "safe-online-evaluation gate passed: candidate detectors stay shadow-only until precision, recall, and burden gates pass"
