#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/modelcheck-gate.json}"
OUT="${2:-results/generated/modelcheck}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.modelcheck-gate/v1" and (.bad_state|type=="string")' "$SPEC" > /dev/null

jq '
  .bad_state as $bad
  | def explore($model):
      $model.transitions as $T
      | { frontier: [[$model.initial]], reached: [$model.initial], traces: [[$model.initial]] }
      # Bounded BFS: at most (#transitions + 1) expansions covers any acyclic reachable path.
      | reduce range(0; ($T|length) + 1) as $_ (.;
          . as $st
          | ([ $st.frontier[]
               | . as $path
               | ($path[-1]) as $tail
               | $T[] | select(.from == $tail) | select([$path[] == .to] | any | not)
               | $path + [.to] ]) as $next
          | {
              frontier: $next,
              reached: (($st.reached + [$next[][-1]]) | unique),
              traces: ($st.traces + $next)
            })
      | {
          reachable: (.reached | unique),
          bad_reachable: ((.reached | index($bad)) != null),
          counterexample: ([ .traces[] | select(.[-1] == $bad) ] | sort_by(length) | .[0])
        };
  {
    version: "patchline.modelcheck/v1",
    bad_state: $bad,
    safe: explore(.safe_model),
    buggy: explore(.buggy_model)
  }
' "$SPEC" > "$OUT/modelcheck.json"

{
  echo "# Model checking of migration rollout"
  echo
  echo "Safe model reaches bad state: $(jq -r '.safe.bad_reachable' "$OUT/modelcheck.json")"
  echo
  echo "Buggy model counterexample: $(jq -rc '.buggy.counterexample' "$OUT/modelcheck.json")"
} > "$OUT/modelcheck.md"
cp "$OUT/modelcheck.md" "$OUT/README.md"

echo "modelcheck worker: safe_bad=$(jq -r '.safe.bad_reachable' "$OUT/modelcheck.json") buggy_cex=$(jq -rc '.buggy.counterexample' "$OUT/modelcheck.json")"
