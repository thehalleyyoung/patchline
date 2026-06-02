#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/pattern-mining-gate.json}"
OUT="${2:-results/generated/pattern-mining}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.pattern-mining-gate/v1" and (.repos|length) >= 1 and (.min_repos|type=="number")' "$SPEC" > /dev/null

# Count distinct repositories per failure mode; promote to recurring at the threshold.
jq '
  .min_repos as $min
  | [ .repos[] | .repo as $r | .failure_modes[] | {mode: ., repo: $r} ]
  | group_by(.mode)
  | map({ mode: .[0].mode, repos: (map(.repo) | unique), repo_count: (map(.repo) | unique | length) })
  | map(. + { recurring: (.repo_count >= $min) })
  | (sort_by([-.repo_count, .mode])) as $ranked
  | {
      version: "patchline.pattern-mining/v1",
      min_repos: $min,
      patterns: $ranked,
      recurring: [ $ranked[] | select(.recurring) ]
    }
' "$SPEC" > "$OUT/pattern-mining.json"

{
  echo "# Cross-repository failure-mode mining"
  echo
  echo "Minimum repositories to count as recurring: $(jq -r .min_repos "$OUT/pattern-mining.json")"
  echo
  echo "| Failure mode | Repos | Recurring |"
  echo "|---|---|---|"
  jq -r '.patterns[] | "| \(.mode) | \(.repo_count) | \(.recurring) |"' "$OUT/pattern-mining.json"
} > "$OUT/pattern-mining.md"
cp "$OUT/pattern-mining.md" "$OUT/README.md"

echo "pattern-mining worker: $(jq -r '.recurring|length' "$OUT/pattern-mining.json") recurring patterns"
