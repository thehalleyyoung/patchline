#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/policy-freeze-gate.json}"
OUT="${2:-results/generated/policy-freeze-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.policy-freeze/v1" and (.claim | length) > 200 and (.organizations | length) >= 2' "$SPEC" > /dev/null
for phrase in "policy-freezing mechanism" "fails closed" "make policy-freeze-gate"; do
  grep -F "$phrase" docs/policy-freeze.md README.md > /dev/null
done

go test ./internal/feedback -run 'TestPolicyFreeze'
go test ./cmd/patchline -run 'TestFeedbackLiveLearningCommandsWriteReports'

go run ./cmd/patchline feedback policy-freeze \
  --spec "$SPEC" \
  --out "$OUT" \
  --json > "$OUT/stdout.json"

test -s "$OUT/policy-freeze.json"
test -s "$OUT/policy-freeze.md"

jq -e '
  .version == "patchline.policy-freeze-report/v1" and
  .ok == true and
  .summary.organizations_evaluated == 2 and
  .summary.pinned_organizations == 1 and
  .summary.allowed_updates == 1 and
  (.decisions | any(.organization == "critical-payments" and .policy_change_allowed == false and .pinned_release == "v1.0.0" and .reason == "active_high_stakes_incident_pins_audited_release")) and
  (.decisions | any(.organization == "platform-sandbox" and .policy_change_allowed == true and .pinned_release == "v1.1.0")) and
  .privacy.source_free == true
' "$OUT/policy-freeze.json" > /dev/null

jq 'del(.audited_releases[0])' "$SPEC" > "$OUT/missing-audit.json"
go run ./cmd/patchline feedback policy-freeze \
  --spec "$OUT/missing-audit.json" \
  --out "$OUT/missing-audit" \
  --json > "$OUT/missing-audit.stdout.json"
jq -e '.ok == false and .summary.blocked_missing_audit >= 1' "$OUT/missing-audit/policy-freeze.json" > /dev/null

if grep -Eiq 'DROP TABLE|UPDATE accounts|diff --git|source_code|raw_evidence|finding_id|evidence_hash' \
  "$OUT/policy-freeze.json" "$OUT/policy-freeze.md"; then
  echo "FAIL: policy-freeze output retained source or raw identifiers" >&2
  exit 1
fi

jq -n --slurpfile r "$OUT/policy-freeze.json" '{
  version: "patchline.policy-freeze-gate-results/v1",
  pinned_organizations: $r[0].summary.pinned_organizations,
  fail_closed_negative_control: true,
  verified: true
}' > "$OUT/gate-summary.json"

echo "policy-freeze gate passed: high-stakes incidents pin audited detector versions and missing audit evidence fails closed"
