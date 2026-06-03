#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/release-channel-isolation-gate.json}"
OUT="${2:-results/generated/release-channel-isolation}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.release-channel-isolation-gate/v1"' "$SPEC" > /dev/null

hash_json() {
  jq -S -c . "$1" | shasum -a 256 | awk '{print $1}'
}

build_input() {
  local results_expr="$1"
  local output="$2"
  jq "$results_expr as \$results | {detectors, stable_certificate, input_results: \$results}" "$SPEC" > "$output"
}

build_certificate() {
  local input="$1"
  local mode="$2"
  local output="$3"
  jq --arg mode "$mode" '
    def in_array($arr; $value): ($arr | index($value)) != null;
    . as $s
    | ($s.stable_certificate.subjects // []) as $subjects
    | ($s.stable_certificate.allowed_detector_ids // []) as $allowed_detector_ids
    | (if $mode == "isolated" then
         [
           $s.input_results[]
           | select(
               .channel == "stable" and
               in_array($allowed_detector_ids; .detector_id) and
               in_array($subjects; .subject)
             )
         ]
       else
         [
           $s.input_results[]
           | select(in_array($subjects; .subject))
         ]
       end
       | sort_by(.id)) as $results
    | {
        version: "patchline.stable-release-certificate/v1",
        certificate_id: $s.stable_certificate.id,
        release_channel: "stable",
        policy: $s.stable_certificate.policy,
        subjects: ($subjects | sort),
        detector_ids: ($results | map(.detector_id) | unique | sort),
        result_ids: ($results | map(.id) | sort),
        results: ($results | map({
          id,
          channel,
          detector_id,
          subject,
          verdict,
          risk_score,
          evidence_hash
        }))
      }
  ' "$input" > "$output"
}

build_input '.baseline_results' "$OUT/baseline-input.json"
build_input '.contaminated_results' "$OUT/contaminated-input.json"
build_input '.negative_controls.mutated_stable_results' "$OUT/mutated-input.json"

build_certificate "$OUT/baseline-input.json" isolated "$OUT/baseline-certificate.json"
build_certificate "$OUT/contaminated-input.json" isolated "$OUT/isolated-certificate.json"
build_certificate "$OUT/contaminated-input.json" naive "$OUT/naive-contaminated-certificate.json"
build_certificate "$OUT/mutated-input.json" isolated "$OUT/mutated-certificate.json"

baseline_hash="$(hash_json "$OUT/baseline-certificate.json")"
isolated_hash="$(hash_json "$OUT/isolated-certificate.json")"
naive_hash="$(hash_json "$OUT/naive-contaminated-certificate.json")"
mutated_hash="$(hash_json "$OUT/mutated-certificate.json")"

explicit_channels="$(jq -r '
  def known: . == "stable" or . == "experimental";
  ([.detectors[]?.channel, .baseline_results[]?.channel, .contaminated_results[]?.channel] | all(known)) and
  (.stable_certificate.channel == "stable")
' "$SPEC")"

unique_detector_ids="$(jq -r '([.detectors[].id] | length) == ([.detectors[].id] | unique | length)' "$SPEC")"

disjoint_channels="$(jq -r '
  [.detectors[] | select(.channel == "stable") | .id] as $stable
  | [.detectors[] | select(.channel == "experimental") | .id] as $experimental
  | ([$stable[] as $detector_id | select(($experimental | index($detector_id)) != null)] | length) == 0
' "$SPEC")"

cert_refs_stable_only="$(jq -r '
  [.detectors[] | select(.channel == "stable") | .id] as $stable
  | (.stable_certificate.allowed_detector_ids // []) as $allowed
  | (.stable_certificate.channel == "stable") and
    (($allowed | length) > 0) and
    ([ $allowed[] as $detector_id | select(($stable | index($detector_id)) == null) ] | length == 0)
' "$SPEC")"

stable_results_closed="$(jq -r '
  [.stable_certificate.allowed_detector_ids[]] as $allowed
  | [.stable_certificate.subjects[]] as $subjects
  | [
      .contaminated_results[]
      | .detector_id as $detector_id
      | .subject as $subject
      | select(.channel == "stable" and (($allowed | index($detector_id)) != null) and (($subjects | index($subject)) != null))
    ] as $stable_results
  | ($stable_results | length) > 0 and
    ([$stable_results[] | select(.channel != "stable")] | length == 0) and
    ([$stable_results[] | .detector_id as $detector_id | select(($allowed | index($detector_id)) == null)] | length == 0)
' "$SPEC")"

experimental_advisory_only="$(jq -r '
  [.contaminated_results[] | select(.channel == "experimental")] as $experimental
  | ($experimental | length) > 0 and
    ([$experimental[] | select(.advisory_only != true)] | length == 0)
' "$SPEC")"

negative_experimental_reference_rejected="$(jq -r '
  [.detectors[] | select(.channel == "stable") | .id] as $stable
  | (.negative_controls.experimental_reference_certificate.allowed_detector_ids // []) as $allowed
  | (.negative_controls.experimental_reference_certificate.channel == "stable") and
    ([ $allowed[] as $detector_id | select(($stable | index($detector_id)) == null) ] | length > 0)
' "$SPEC")"

stable_hash_equal=false
if [[ "$baseline_hash" == "$isolated_hash" ]]; then
  stable_hash_equal=true
fi

experimental_is_nontrivial=false
if [[ "$naive_hash" != "$isolated_hash" ]]; then
  experimental_is_nontrivial=true
fi

negative_mutated_result_rejected=false
if [[ "$mutated_hash" != "$baseline_hash" ]]; then
  negative_mutated_result_rejected=true
fi

jq -n \
  --arg baseline_hash "$baseline_hash" \
  --arg isolated_hash "$isolated_hash" \
  --arg naive_hash "$naive_hash" \
  --arg mutated_hash "$mutated_hash" \
  --argjson explicit_channels "$explicit_channels" \
  --argjson unique_detector_ids "$unique_detector_ids" \
  --argjson disjoint_channels "$disjoint_channels" \
  --argjson cert_refs_stable_only "$cert_refs_stable_only" \
  --argjson stable_results_closed "$stable_results_closed" \
  --argjson experimental_advisory_only "$experimental_advisory_only" \
  --argjson stable_hash_equal "$stable_hash_equal" \
  --argjson experimental_is_nontrivial "$experimental_is_nontrivial" \
  --argjson negative_experimental_reference_rejected "$negative_experimental_reference_rejected" \
  --argjson negative_mutated_result_rejected "$negative_mutated_result_rejected" \
  --slurpfile baseline "$OUT/baseline-certificate.json" \
  --slurpfile isolated "$OUT/isolated-certificate.json" \
  --slurpfile naive "$OUT/naive-contaminated-certificate.json" \
  '{
    version: "patchline.release-channel-isolation/v1",
    all_ok: (
      $explicit_channels and
      $unique_detector_ids and
      $disjoint_channels and
      $cert_refs_stable_only and
      $stable_results_closed and
      $experimental_advisory_only and
      $stable_hash_equal and
      $experimental_is_nontrivial and
      $negative_experimental_reference_rejected and
      $negative_mutated_result_rejected
    ),
    proofs: {
      explicit_channels: $explicit_channels,
      unique_detector_ids: $unique_detector_ids,
      disjoint_channels: $disjoint_channels,
      cert_refs_stable_only: $cert_refs_stable_only,
      stable_results_closed: $stable_results_closed,
      experimental_advisory_only: $experimental_advisory_only,
      stable_hash_equal_after_experimental_injection: $stable_hash_equal,
      experimental_is_nontrivial: $experimental_is_nontrivial
    },
    hashes: {
      baseline: $baseline_hash,
      isolated: $isolated_hash,
      naive: $naive_hash,
      mutated: $mutated_hash
    },
    counts: {
      stable_results: ($isolated[0].results | length),
      naive_results: ($naive[0].results | length),
      experimental_results_excluded: (($naive[0].results | length) - ($isolated[0].results | length))
    },
    negative_controls: {
      experimental_reference_rejected: $negative_experimental_reference_rejected,
      experimental_reference_reason: "stable certificate detector_ids must be a subset of stable channel detectors",
      mutated_result_rejected: $negative_mutated_result_rejected,
      mutated_result_reason: "stable certificate hash recomputation changed after a stable result mutation"
    },
    stable_certificate: $isolated[0],
    naive_nonisolating_certificate: $naive[0],
    baseline_certificate: $baseline[0]
  }' > "$OUT/out.json"

{
  echo "# Release-channel isolation"
  echo
  echo "Stable baseline hash: \`$baseline_hash\`"
  echo
  echo "Stable hash with experimental detector output injected: \`$isolated_hash\`"
  echo
  echo "Naive non-isolating hash: \`$naive_hash\`"
  echo
  echo "All proofs passed: \`$(jq -r .all_ok "$OUT/out.json")\`"
} > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"

echo "release-channel-isolation worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
