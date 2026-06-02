#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/failure-injection-suite-gate.json}"
OUT="${2:-results/generated/failure-injection-suite-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '
  .version == "patchline.failure-injection-suite-gate/v1" and
  (.required_probe_kinds | sort) == ["cache-drift","generated-evidence-drift","ref-drift"] and
  (.required_outputs | length) >= 6
' "$SPEC" > /dev/null

for phrase in "failure-injection suite" "fail loudly" "refs" "caches" "generated evidence" "make failure-injection-suite-gate"; do
  grep -F "$phrase" docs/failure-injection-suite.md README.md > /dev/null
done

bash scripts/failure-injection-suite.sh "$SPEC" "$OUT" > "$OUT.run.log"

while read -r output; do
  test -s "$OUT/$output"
done < <(jq -r '.required_outputs[]' "$SPEC")

min_probes="$(jq '.minimum_probes' "$SPEC")"
min_repos="$(jq '.minimum_public_repos' "$SPEC")"
required_kinds="$(jq -c '.required_probe_kinds' "$SPEC")"
jq -e --argjson min_probes "$min_probes" --argjson min_repos "$min_repos" --argjson required_kinds "$required_kinds" '
  . as $root |
  .version == "patchline.failure-injection-suite/v1" and
  .summary.probes >= $min_probes and
  .summary.failed_loudly == .summary.probes and
  .summary.public_repos >= $min_repos and
  .summary.ranked_risks > 100 and
  .summary.verified == true and
  all($required_kinds[]; . as $kind | any($root.probes[]; .kind == $kind and .failed_loudly == true and .exit_code != 0))
' "$OUT/failure-injection-results.json" > /dev/null

grep -F "ref drift rejected with exit" "$OUT/probes/ref-drift.log" > /dev/null
grep -F "cache drift rejected with exit" "$OUT/probes/cache-drift.log" > /dev/null
grep -F "generated evidence drift rejected with exit" "$OUT/probes/generated-evidence-drift.log" > /dev/null
grep -F "invalid public ref" "$OUT/failure-injection-results.json" > /dev/null
grep -F "cached public archive checksum drift" "$OUT/failure-injection-results.json" > /dev/null
grep -F "generated public-code evidence threshold drift" "$OUT/failure-injection-results.json" > /dev/null
grep -F "Failure-injection suite" "$OUT/failure-injection-results.md" > /dev/null

jq -n \
  --slurpfile results "$OUT/failure-injection-results.json" \
  '{
    version:"patchline.failure-injection-suite-gate-results/v1",
    probes:$results[0].summary.probes,
    failed_loudly:$results[0].summary.failed_loudly,
    public_repos:$results[0].summary.public_repos,
    ranked_risks:$results[0].summary.ranked_risks,
    verified:true
  }' > "$OUT/gate-summary.json"

echo "failure injection suite gate passed: probes $(jq '.probes' "$OUT/gate-summary.json"), failed loudly $(jq '.failed_loudly' "$OUT/gate-summary.json"), risks $(jq '.ranked_risks' "$OUT/gate-summary.json")"
