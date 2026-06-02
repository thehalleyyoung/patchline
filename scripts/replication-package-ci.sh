#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/replication-package-ci-gate.json}"; OUT="${2:-results/generated/replication-package-ci}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.replication-package-ci-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.reran and .identical) ]|length) as $ok
  | {version:"patchline.replication-package-ci/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.reran and .identical))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Replication package across CI providers"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "replication-package-ci worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
