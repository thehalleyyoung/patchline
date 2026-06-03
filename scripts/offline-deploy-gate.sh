#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/offline-deploy.json}"
OUT="${2:-results/generated/offline-deploy-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.offline-deploy/v1" and
  (.claim | length) > 300 and
  (.profiles | length) == 2 and
  ([.profiles[].environment] | sort) == ["clinic-edge","finance-review-room"] and
  .criteria.require_no_network == true and
  .criteria.require_telemetry_disabled == true and
  .criteria.require_pinned_bundles == true and
  .criteria.require_pinned_update_bundles == true and
  .criteria.require_offline_updates == true and
  .criteria.require_rollback_plan == true
' "$SPEC" > /dev/null

for phrase in "Reproducible edge/offline deployment" "no network, no telemetry" "pinned update bundles" "make offline-deploy-gate"; do
  grep -F "$phrase" docs/offline-deploy.md README.md > /dev/null
done

go test ./internal/offlinedeploy -run 'TestBuildReport|TestReadSpec|TestWriteArtifacts' -count=1
go test ./cmd/patchline -run TestOfflineDeployCommandWritesReports -count=1

go run ./cmd/patchline offline-deploy \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/offline-deploy.json"
test -s "$OUT/safe/offline-deploy.md"
shasum -a 256 -c examples/offline-deploy/updates/clinic/MANIFEST.checks > /dev/null
shasum -a 256 -c examples/offline-deploy/updates/finance/MANIFEST.checks > /dev/null

jq -e '
  .version == "patchline.offline-deploy-report/v1" and
  .ok == true and
  .summary.profiles == 2 and
  .summary.no_network_profiles == 2 and
  .summary.telemetry_disabled_profiles == 2 and
  .summary.bundles == 4 and
  .summary.pinned_bundles == 4 and
  .summary.update_bundles == 2 and
  .summary.offline_update_bundles == 2 and
  .summary.rollback_plans == 2 and
  ([.profiles[].bundles[].artifact.sha256 | select(test("^sha256:[0-9a-f]{64}$"))] | length) == 4 and
  ([.profiles[].update_bundles[].artifact.sha256 | select(test("^sha256:[0-9a-f]{64}$"))] | length) == 2
' "$OUT/safe/offline-deploy.json" > /dev/null

jq '
  .profiles = .profiles[:1] |
  .profiles[0].network_policy.mode = "internet" |
  .profiles[0].network_policy.egress_allowed = true |
  .profiles[0].network_policy.allowed_endpoints = ["updates.patchline.example"] |
  .profiles[0].telemetry_policy.mode = "enabled" |
  .profiles[0].telemetry_policy.enabled = true |
  .profiles[0].telemetry_policy.destinations = ["https://otel.patchline.example/v1/traces"] |
  .profiles[0].install_commands += ["curl https://updates.patchline.example/patchline.tar -o patchline.tar"] |
  .profiles[0].verify_commands += ["patchline --telemetry=enabled"] |
  .profiles[0].bundles[0].sha256 = "sha256:0000000000000000000000000000000000000000000000000000000000000000" |
  .profiles[0].bundles[1].signature_path = "" |
  .profiles[0].bundles[1].sbom_path = "" |
  .profiles[0].update_bundles[0].sha256 = "" |
  .profiles[0].update_bundles[0].offline = false |
  .profiles[0].update_bundles[0].manifest_path = "" |
  .profiles[0].rollback_plan.id = "" |
  .profiles[0].rollback_plan.previous_bundle_sha256 = "" |
  .profiles[0].rollback_plan.commands = []
' "$SPEC" > "$OUT/bad-spec.json"

set +e
go run ./cmd/patchline offline-deploy \
  --spec "$OUT/bad-spec.json" \
  --root . \
  --out "$OUT/bad" \
  --json > "$OUT/bad.stdout.json"
bad_status=$?
set -e
if [[ "$bad_status" -eq 0 ]]; then
  echo "FAIL: bad offline deployment spec unexpectedly exited successfully" >&2
  exit 1
fi

jq -e '
  .ok == false and
  (.counterexamples | any(.kind == "missing_required_environment")) and
  (.counterexamples | any(.kind == "network_not_disabled")) and
  (.counterexamples | any(.kind == "network_command")) and
  (.counterexamples | any(.kind == "telemetry_enabled")) and
  (.counterexamples | any(.kind == "telemetry_command")) and
  (.counterexamples | any(.kind == "bundle_hash_mismatch")) and
  (.counterexamples | any(.kind == "missing_signature")) and
  (.counterexamples | any(.kind == "missing_sbom")) and
  (.counterexamples | any(.kind == "unpinned_update_bundle")) and
  (.counterexamples | any(.kind == "missing_update_manifest")) and
  (.counterexamples | any(.kind == "update_not_offline")) and
  (.counterexamples | any(.kind == "missing_rollback_plan"))
' "$OUT/bad/offline-deploy.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/offline-deploy.json")"
go run ./cmd/patchline offline-deploy \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/offline-deploy.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: offline deployment report hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/offline-deploy.json" --slurpfile bad "$OUT/bad/offline-deploy.json" '{
  version: "patchline.offline-deploy-gate-results/v1",
  profiles: $safe[0].summary.profiles,
  pinned_bundles: $safe[0].summary.pinned_bundles,
  offline_update_bundles: $safe[0].summary.offline_update_bundles,
  rollback_plans: $safe[0].summary.rollback_plans,
  bad_counterexamples: $bad[0].counterexamples,
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "offline deployment gate passed: regulated profiles are no-network, no-telemetry, bundle-pinned, rollback-ready, and deterministic"
