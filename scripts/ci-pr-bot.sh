#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/ci-pr-bot-gate.json}"; OUT="${2:-results/generated/ci-pr-bot}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.ci-pr-bot-gate/v1"' "$SPEC" > /dev/null
jq '

  {version:"patchline.ci-pr-bot/v1",
   idempotent:(.run_a_hash == .run_b_hash),
   anchored:((.anchor|length)>0),
   body_match:(.run_a_body == .run_b_body),
   changed_updates:(.changed_diff_hash != .run_a_hash)}

' "$SPEC" > "$OUT/out.json"
{ echo "# CI pull-request bot"; echo; echo "Idempotent $(jq -r .idempotent "$OUT/out.json"); anchored $(jq -r .anchored "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "ci-pr-bot worker: idempotent=$(jq -r .idempotent "$OUT/out.json")"
