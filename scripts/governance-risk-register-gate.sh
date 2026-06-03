#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/governance-risk-register.json}"
OUT="${2:-results/generated/governance-risk-register-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.governance-risk-register/v1" and
  .as_of_date == "2026-03-01T00:00:00Z" and
  (.criteria.required_domains | sort) == (["benchmark_control","funding","infrastructure","maintainership"] | sort) and
  .criteria.max_owner_share == 0.6 and
  .criteria.max_organization_share == 0.65 and
  .criteria.require_evidence_paths == true and
  .criteria.require_rotation_plan == true and
  (.entries | length) == 12
' "$SPEC" > /dev/null

for path in $(jq -r '.entries[].evidence_paths[]?' "$SPEC" | sort -u); do
  test -s "$path"
done

for phrase in "Governance-risk register" "governance-risk-register" "make governance-risk-register-gate"; do
  grep -F "$phrase" docs/governance-risk-register.md README.md > /dev/null
done

go test ./internal/governancerisk -run 'TestBuildReport|TestReadSpec'
go test ./cmd/patchline -run TestGovernanceRiskRegisterCommandWritesReports

go run ./cmd/patchline governance-risk-register \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/governance-risk-register.json"
test -s "$OUT/safe/governance-risk-register.md"

jq -e '
  .version == "patchline.governance-risk-register-report/v1" and
  .ok == true and
  .summary.domains == 4 and
  .summary.entries == 12 and
  .summary.high_risk_domains == 0 and
  .summary.max_owner_share <= 0.6 and
  .summary.max_organization_share <= 0.65 and
  .summary.counterexamples == 0 and
  ([.domains[] | select(.domain == "benchmark_control" and (.evidence | length >= 2))] | length) == 1 and
  ([.domains[].evidence[] | select(.sha256 | length == 64)] | length) >= 6
' "$OUT/safe/governance-risk-register.json" > /dev/null

jq '
  (.entries[] | select(.domain == "maintainership") | .owner) = "single maintainer" |
  (.entries[] | select(.domain == "maintainership") | .organization) = "single maintainer org"
' "$SPEC" > "$OUT/concentrated-maintainership.json"

go run ./cmd/patchline governance-risk-register \
  --spec "$OUT/concentrated-maintainership.json" \
  --root . \
  --out "$OUT/concentrated" \
  --json > "$OUT/concentrated.stdout.json"

jq -e '
  .ok == false and
  ([.counterexamples[] | select(.kind == "owner_share_exceeded" and .subject == "maintainership")] | length) == 1 and
  ([.counterexamples[] | select(.kind == "organization_share_exceeded" and .subject == "maintainership")] | length) == 1 and
  ([.counterexamples[] | select(.kind == "insufficient_independent_owners" and .subject == "maintainership")] | length) == 1
' "$OUT/concentrated/governance-risk-register.json" > /dev/null

jq '
  (.entries[] | select(.asset_id == "maint-core-release") | .last_reviewed) = "2025-01-01T00:00:00Z" |
  (.entries[] | select(.asset_id == "maint-core-release") | .evidence_paths) = ["../outside.md"]
' "$SPEC" > "$OUT/stale-escaped-evidence.json"

go run ./cmd/patchline governance-risk-register \
  --spec "$OUT/stale-escaped-evidence.json" \
  --root . \
  --out "$OUT/stale" \
  --json > "$OUT/stale.stdout.json"

jq -e '
  .ok == false and
  ([.counterexamples[] | select(.kind == "stale_review" and .subject == "maint-core-release")] | length) == 1 and
  ([.counterexamples[] | select(.kind == "invalid_evidence_path" and .subject == "maint-core-release")] | length) == 1 and
  ([.counterexamples[] | select(.kind == "missing_entry_evidence" and .subject == "maint-core-release")] | length) == 1
' "$OUT/stale/governance-risk-register.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/governance-risk-register.json")"
go run ./cmd/patchline governance-risk-register \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/governance-risk-register.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: governance-risk register hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/governance-risk-register.json" --slurpfile concentrated "$OUT/concentrated/governance-risk-register.json" --slurpfile stale "$OUT/stale/governance-risk-register.json" '{
  version: "patchline.governance-risk-register-gate-results/v1",
  domains: $safe[0].summary.domains,
  entries: $safe[0].summary.entries,
  max_owner_share: $safe[0].summary.max_owner_share,
  max_organization_share: $safe[0].summary.max_organization_share,
  concentration_negative_control: [$concentrated[0].counterexamples[].kind],
  stale_evidence_negative_control: [$stale[0].counterexamples[].kind],
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "governance-risk register gate passed: maintainership, funding, infrastructure, and benchmark-control concentration are audited with hashed evidence and negative controls"
