#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/failure-injection-suite-gate.json}"
OUT="${2:-results/generated/failure-injection-suite}"
rm -rf "$OUT"
mkdir -p "$OUT/evidence" "$OUT/probes"

jq -e '
  .version == "patchline.failure-injection-suite-gate/v1" and
  (.claim | length) > 120 and
  (.required_probe_kinds | sort) == ["cache-drift","generated-evidence-drift","ref-drift"] and
  (.required_outputs | length) >= 6
' "$SPEC" > /dev/null

replication_spec="$(jq -r '.independent_replication_spec' "$SPEC")"
bash scripts/generate-independent-replication.sh "$replication_spec" "$OUT/evidence/independent-replication" > "$OUT/evidence/independent-replication.run.log"

replication="$OUT/evidence/independent-replication/independent-replication.json"
test -s "$replication"

probe_rows=()

bad_ref_spec="$OUT/probes/bad-ref-capstone.json"
jq '.real_code[0].ref = "not-a-40-character-sha"' examples/capstone-demo-gate.json > "$bad_ref_spec"
ref_log="$OUT/probes/ref-drift.log"
if bash scripts/capstone-demo-gate.sh "$bad_ref_spec" "$OUT/probes/ref-drift-out" > "$ref_log" 2>&1; then
  echo "ref drift unexpectedly passed" >> "$ref_log"
  ref_exit=0
else
  ref_exit=$?
  echo "ref drift rejected with exit $ref_exit" >> "$ref_log"
fi
jq -n \
  --arg kind "ref-drift" \
  --arg target "scripts/capstone-demo-gate.sh" \
  --arg log "$ref_log" \
  --argjson exit_code "$ref_exit" \
  '{kind:$kind,target:$target,log:$log,exit_code:$exit_code,failed_loudly:($exit_code != 0),message:"invalid public ref was rejected before release evidence could be trusted"}' > "$OUT/probes/ref-drift.json"
probe_rows+=("$OUT/probes/ref-drift.json")

cache_probe="$OUT/probes/cache-drift.json"
jq '.archives[0].expected_hash = "sha256:0000000000000000000000000000000000000000000000000000000000000000"' "$OUT/evidence/independent-replication/archive-verification.json" > "$cache_probe"
cache_log="$OUT/probes/cache-drift.log"
if jq -e 'all(.archives[]; .verified == true and .expected_hash == .actual_hash)' "$cache_probe" > "$cache_log" 2>&1; then
  echo "cache drift unexpectedly passed" >> "$cache_log"
  cache_exit=0
else
  cache_exit=$?
  echo "cache drift rejected with exit $cache_exit" >> "$cache_log"
fi
jq -n \
  --arg kind "cache-drift" \
  --arg target "archive-verification.json" \
  --arg log "$cache_log" \
  --argjson exit_code "$cache_exit" \
  '{kind:$kind,target:$target,log:$log,exit_code:$exit_code,failed_loudly:($exit_code != 0),message:"cached public archive checksum drift was rejected"}' > "$OUT/probes/cache-drift-result.json"
probe_rows+=("$OUT/probes/cache-drift-result.json")

evidence_probe="$OUT/probes/generated-evidence-drift.json"
jq '.summary.ranked_risks = 0 | .summary.verified = false' "$replication" > "$evidence_probe"
evidence_log="$OUT/probes/generated-evidence-drift.log"
if jq -e '.summary.ranked_risks > 100 and .summary.verified == true and .summary.public_repos >= 4' "$evidence_probe" > "$evidence_log" 2>&1; then
  echo "generated evidence drift unexpectedly passed" >> "$evidence_log"
  evidence_exit=0
else
  evidence_exit=$?
  echo "generated evidence drift rejected with exit $evidence_exit" >> "$evidence_log"
fi
jq -n \
  --arg kind "generated-evidence-drift" \
  --arg target "independent-replication.json" \
  --arg log "$evidence_log" \
  --argjson exit_code "$evidence_exit" \
  '{kind:$kind,target:$target,log:$log,exit_code:$exit_code,failed_loudly:($exit_code != 0),message:"generated public-code evidence threshold drift was rejected"}' > "$OUT/probes/generated-evidence-drift-result.json"
probe_rows+=("$OUT/probes/generated-evidence-drift-result.json")

jq -n \
  --slurpfile probes <(jq -s '.' "${probe_rows[@]}") \
  --slurpfile replication "$replication" \
  '{
    version:"patchline.failure-injection-suite/v1",
    probes:$probes[0],
    evidence:{
      independent_replication:"evidence/independent-replication/independent-replication.json",
      archive_verification:"evidence/independent-replication/archive-verification.json",
      release_manifest:"evidence/independent-replication/evidence/release-manifest/artifact-release-manifest.json"
    },
    summary:{
      probes:($probes[0] | length),
      failed_loudly:($probes[0] | map(select(.failed_loudly == true)) | length),
      public_repos:$replication[0].summary.public_repos,
      ranked_risks:$replication[0].summary.ranked_risks,
      verified:($replication[0].summary.verified == true and all($probes[0][]; .failed_loudly == true))
    }
  }' > "$OUT/failure-injection-results.json"

{
  echo "# Failure-injection suite"
  echo
  echo "This suite proves artifact gates fail loudly when refs, caches, or generated evidence drift."
  echo
  echo "## Probe results"
  jq -r '.probes[] | "- `" + .kind + "` against `" + .target + "`: failed loudly=`" + (.failed_loudly|tostring) + "`, exit=`" + (.exit_code|tostring) + "`. " + .message' "$OUT/failure-injection-results.json"
  echo
  echo "## Public-code evidence baseline"
  jq -r '.summary | "- public repositories: `" + (.public_repos|tostring) + "`\n- ranked risks: `" + (.ranked_risks|tostring) + "`\n- probes: `" + (.probes|tostring) + "`\n- failed loudly: `" + (.failed_loudly|tostring) + "`"' "$OUT/failure-injection-results.json"
} > "$OUT/failure-injection-results.md"

cp "$OUT/failure-injection-results.md" "$OUT/README.md"
echo "failure injection suite complete: probes $(jq '.summary.probes' "$OUT/failure-injection-results.json"), failed loudly $(jq '.summary.failed_loudly' "$OUT/failure-injection-results.json")"
