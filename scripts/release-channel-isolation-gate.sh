#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/release-channel-isolation-gate.json}"
OUT="${2:-results/generated/release-channel-isolation}"

mkdir -p "$(dirname "$OUT")"

jq -e '
  .version == "patchline.release-channel-isolation-gate/v1" and
  (.claim | length) > 300 and
  (.channels | length) == 2 and
  ([.channels[].id] | sort) == ["experimental", "stable"] and
  (.detectors | length) >= 5 and
  (.baseline_results | length) >= 3 and
  (.contaminated_results | length) > (.baseline_results | length)
' "$SPEC" > /dev/null

for phrase in "Release-channel isolation" "stable certificate results" "make release-channel-isolation-gate"; do
  grep -F "$phrase" docs/release-channel-isolation.md README.md > /dev/null
done

bash scripts/release-channel-isolation.sh "$SPEC" "$OUT" > "$OUT.run.log"

jq -e '
  .version == "patchline.release-channel-isolation/v1" and
  .all_ok == true and
  .proofs.explicit_channels == true and
  .proofs.unique_detector_ids == true and
  .proofs.disjoint_channels == true and
  .proofs.cert_refs_stable_only == true and
  .proofs.stable_results_closed == true and
  .proofs.experimental_advisory_only == true and
  .proofs.stable_hash_equal_after_experimental_injection == true and
  .proofs.experimental_is_nontrivial == true and
  .negative_controls.experimental_reference_rejected == true and
  .negative_controls.mutated_result_rejected == true and
  .hashes.baseline == .hashes.isolated and
  .hashes.naive != .hashes.isolated and
  .hashes.mutated != .hashes.baseline and
  .counts.experimental_results_excluded >= 1
' "$OUT/out.json" > /dev/null

jq -n --slurpfile r "$OUT/out.json" '{
  version: "patchline.release-channel-isolation-gate-results/v1",
  verified: true,
  stable_hash: $r[0].hashes.baseline,
  isolated_hash: $r[0].hashes.isolated,
  naive_hash: $r[0].hashes.naive,
  experimental_results_excluded: $r[0].counts.experimental_results_excluded,
  proofs: $r[0].proofs,
  negative_controls: $r[0].negative_controls
}' > "$OUT/gate-summary.json"

echo "release-channel-isolation gate passed: experimental detectors changed the naive hash but not the stable certificate hash"
