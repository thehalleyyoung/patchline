#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/accelerator-fallbacks.json}"
OUT="${2:-results/generated/accelerator-fallbacks-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.accelerator-fallbacks/v1" and
  (.claim | length) > 500 and
  (.components | length) == 27 and
  (.criteria.required_component_ids | length) == 27 and
  .criteria.require_repository_discovery == true and
  .criteria.require_cpu_fallback == true and
  .criteria.require_accelerator_free == true and
  .criteria.require_no_network == true and
  .criteria.require_deterministic_replay == true and
  .criteria.require_stable_seed == true and
  .criteria.require_pinned_learned_artifact == true and
  .criteria.require_pinned_implementation == true and
  .criteria.require_pinned_inputs == true and
  .criteria.require_pinned_outputs == true and
  .criteria.require_parity_evidence == true and
  ([.components[].fallback.gpu_required] | all(. == false)) and
  ([.components[].fallback.accelerator_required] | all(. == false)) and
  ([.components[].fallback.network_allowed] | all(. == false)) and
  ([.components[].fallback.replay_evidence_paths | length] | all(. == 2))
' "$SPEC" > /dev/null

for phrase in "Deterministic accelerator-free fallbacks" "repository-discovered learned component" "make accelerator-fallbacks-gate"; do
  grep -F "$phrase" docs/accelerator-fallbacks.md README.md > /dev/null
done

go test ./internal/acceleratorfallback -count=1
go test ./cmd/patchline -run TestAcceleratorFallbacksCommandWritesReports -count=1

go run ./cmd/patchline accelerator-fallbacks \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/accelerator-fallbacks.json"
test -s "$OUT/safe/accelerator-fallbacks.md"

jq -e '
  .version == "patchline.accelerator-fallbacks-report/v1" and
  .ok == true and
  .summary.discovered_components == 27 and
  .summary.components == 27 and
  .summary.cpu_fallbacks == 27 and
  .summary.accelerator_free_fallbacks == 27 and
  .summary.no_network_fallbacks == 27 and
  .summary.deterministic_fallbacks == 27 and
  .summary.replay_proofs == 54 and
  .summary.parity_checks == 27 and
  .summary.counterexamples == 0 and
  ([.components[].fallback.runtime] | all(. == "go-cpu")) and
  ([.components[].fallback.replay_evidence | length] | all(. == 2))
' "$OUT/safe/accelerator-fallbacks.json" > /dev/null

jq '
  .components |= .[1:] |
  .components[0].fallback.gpu_required = true |
  .components[0].fallback.replay_evidence_paths = ["examples/accelerator-fallbacks/replay/pass-1.jsonl"] |
  .components[1].fallback.input_artifacts[0].sha256 = "sha256:0000000000000000000000000000000000000000000000000000000000000000" |
  .components[2].fallback.replay_evidence_paths = ["examples/accelerator-fallbacks/replay/pass-1.jsonl", "examples/accelerator-fallbacks/replay/nondeterministic.jsonl"] |
  .components[3].parity.fallback_value = 0.50
' "$SPEC" > "$OUT/bad-spec.json"

set +e
go run ./cmd/patchline accelerator-fallbacks \
  --spec "$OUT/bad-spec.json" \
  --root . \
  --out "$OUT/bad" \
  --json > "$OUT/bad.stdout.json" 2> "$OUT/bad.stderr.txt"
bad_status=$?
set -e
if [[ "$bad_status" -eq 0 ]]; then
  echo "FAIL: bad accelerator fallback spec unexpectedly exited successfully" >&2
  exit 1
fi

jq -e '
  .ok == false and
  (.counterexamples | any(.kind == "missing_inventory_component")) and
  (.counterexamples | any(.kind == "missing_required_component")) and
  (.counterexamples | any(.kind == "gpu_required")) and
  (.counterexamples | any(.kind == "missing_replay_evidence")) and
  (.counterexamples | any(.kind == "input_artifact_hash_mismatch")) and
  (.counterexamples | any(.kind == "nondeterministic_replay")) and
  (.counterexamples | any(.kind == "replay_missing_component_output")) and
  (.counterexamples | any(.kind == "parity_drift_exceeds_bound"))
' "$OUT/bad/accelerator-fallbacks.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/accelerator-fallbacks.json")"
go run ./cmd/patchline accelerator-fallbacks \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/accelerator-fallbacks.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: accelerator fallback report hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/accelerator-fallbacks.json" --slurpfile bad "$OUT/bad/accelerator-fallbacks.json" '{
  version: "patchline.accelerator-fallbacks-gate-results/v1",
  discovered_components: $safe[0].summary.discovered_components,
  cpu_fallbacks: $safe[0].summary.cpu_fallbacks,
  replay_proofs: $safe[0].summary.replay_proofs,
  parity_checks: $safe[0].summary.parity_checks,
  bad_counterexamples: $bad[0].counterexamples,
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "accelerator fallback gate passed: 27 learned components covered by deterministic CPU-only fallbacks"
