#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/incident-simulation-env-gate.json}"; OUT="${2:-results/generated/incident-simulation-env}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.incident-simulation-env-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.timeline_valid) ]|length) as $ok
  | {version:"patchline.incident-simulation-env/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.timeline_valid))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Synthetic incident-timeline simulation"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "incident-simulation-env worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
