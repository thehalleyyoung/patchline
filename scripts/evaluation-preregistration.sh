#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/evaluation-preregistration-gate.json}"; OUT="${2:-results/generated/evaluation-preregistration}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.evaluation-preregistration-gate/v1"' "$SPEC" > /dev/null
jq '

  {version:"patchline.evaluation-preregistration/v1",
   matches:(.preregistered_hash == .executed_hash),
   altered_matches:(.preregistered_hash == .altered_hash)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Evaluation pre-registration"; echo; echo "Matches $(jq -r .matches "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "evaluation-preregistration worker: matches=$(jq -r .matches "$OUT/out.json")"
