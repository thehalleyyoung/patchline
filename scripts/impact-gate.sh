#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATES="${1:-examples/feature-impact-gates.json}"
SLICES="${2:-examples/real-repo-slices.json}"

jq -e '
  .version == "patchline.feature-impact-gates/v1" and
  (.features | length) > 0 and
  all(.features[];
    (.id | length) > 0 and
    (.feature | length) > 0 and
    (.real_repo | length) > 0 and
    (.subpath | length) > 0 and
    (.real_repo_failure_mode | length) > 20
  )
' "$GATES" > /dev/null

jq -n --slurpfile gates "$GATES" --slurpfile slices "$SLICES" '
  ($slices[0].slices | map(.repo + ":" + .subpath)) as $known |
  $gates[0].features
  | all(.[]; (.real_repo + ":" + .subpath) as $key | ($known | index($key)))
' | jq -e '. == true' > /dev/null

echo "impact gate passed: $(jq '.features | length' "$GATES") feature entries name real-repo failure modes"
