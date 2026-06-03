#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/resource-profiles.json}"
OUT="${2:-results/generated/resource-profiles-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.resource-profiles/v1" and
  (.claim | length) > 450 and
  (.profiles | length) == 4 and
  (.criteria.required_profile_ids | length) == 4 and
  (.criteria.required_tiers | length) == 4 and
  .criteria.require_deterministic == true and
  .criteria.require_no_network_when_offline == true and
  .criteria.require_cache_strategy == true and
  .criteria.require_native_test_policy == true and
  .criteria.require_graceful_degradation == true and
  ([.profiles[].commands | length] | all(. >= 2)) and
  ([.profiles[].budgets | length] | all(. >= 1)) and
  ([.profiles[].commands[].args[0]] | all(. == "patchline")) and
  ([.profiles[].constraints.llm_allowed] | all(. == false))
' "$SPEC" > /dev/null

for phrase in "Resource-adaptive analysis profiles" "laptops, CI runners, air-gapped servers" "make resource-profiles-gate"; do
  grep -F "$phrase" docs/resource-profiles.md README.md > /dev/null
done

go test ./internal/resourceprofile -count=1
go test ./cmd/patchline -run TestResourceProfilesCommandWritesReports -count=1

go run ./cmd/patchline resource-profiles \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/resource-profiles.json"
test -s "$OUT/safe/resource-profiles.md"

jq -e '
  .version == "patchline.resource-profiles-report/v1" and
  .ok == true and
  .summary.profiles == 4 and
  .summary.tiers == 4 and
  .summary.laptop_profiles == 1 and
  .summary.ci_profiles == 1 and
  .summary.offline_profiles == 1 and
  .summary.hosted_profiles == 1 and
  .summary.deterministic_profiles == 4 and
  .summary.cached_profiles == 4 and
  .summary.native_test_policies == 4 and
  .summary.graceful_degradations == 4 and
  .summary.command_plans == 8 and
  .summary.budgets == 4 and
  .summary.counterexamples == 0
' "$OUT/safe/resource-profiles.json" > /dev/null

jq '
  .profiles |= [.[0], .[2], .[3]] |
  .profiles[0].constraints.cpu = 9 |
  .profiles[0].commands[0].args += ["--llm-command", "remote-generator"] |
  .profiles[1].constraints.network_allowed = true |
  .profiles[1].cache_strategy = "" |
  .profiles[2].constraints.max_cost_cents = 99 |
  .profiles[2].budgets[0].tokens = 0 |
  .profiles[2].evidence_paths = []
' "$SPEC" > "$OUT/bad-spec.json"

set +e
go run ./cmd/patchline resource-profiles \
  --spec "$OUT/bad-spec.json" \
  --root . \
  --out "$OUT/bad" \
  --json > "$OUT/bad.stdout.json" 2> "$OUT/bad.stderr.txt"
bad_status=$?
set -e
if [[ "$bad_status" -eq 0 ]]; then
  echo "FAIL: bad resource profile spec unexpectedly exited successfully" >&2
  exit 1
fi

jq -e '
  .ok == false and
  (.counterexamples | any(.kind == "missing_required_profile")) and
  (.counterexamples | any(.kind == "missing_required_tier")) and
  (.counterexamples | any(.kind == "cpu_budget_exceeded")) and
  (.counterexamples | any(.kind == "nondeterministic_profile")) and
  (.counterexamples | any(.kind == "llm_not_allowed")) and
  (.counterexamples | any(.kind == "offline_network_allowed")) and
  (.counterexamples | any(.kind == "missing_cache_strategy")) and
  (.counterexamples | any(.kind == "cost_budget_exceeded")) and
  (.counterexamples | any(.kind == "invalid_budget")) and
  (.counterexamples | any(.kind == "missing_profile_evidence"))
' "$OUT/bad/resource-profiles.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/resource-profiles.json")"
go run ./cmd/patchline resource-profiles \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/resource-profiles.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: resource profile report hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/resource-profiles.json" --slurpfile bad "$OUT/bad/resource-profiles.json" '{
  version: "patchline.resource-profiles-gate-results/v1",
  profiles: $safe[0].summary.profiles,
  tiers: $safe[0].summary.tiers,
  command_plans: $safe[0].summary.command_plans,
  budgets: $safe[0].summary.budgets,
  bad_counterexamples: $bad[0].counterexamples,
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "resource profiles gate passed: laptop, CI, air-gapped, and hosted profiles verified"
