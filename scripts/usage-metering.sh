#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/usage-metering-gate.json}"; OUT="${2:-results/generated/usage-metering}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.usage-metering-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.reproduced_from_events) ]|length) as $ok
  | {version:"patchline.usage-metering/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.reproduced_from_events))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Billing-and-usage metering with reproducible invoices"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "usage-metering worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
