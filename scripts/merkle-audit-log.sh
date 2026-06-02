#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/merkle-audit-log-gate.json}"; OUT="${2:-results/generated/merkle-audit-log}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.merkle-audit-log-gate/v1"' "$SPEC" > /dev/null
jq '

  .entries as $e | .tampered as $t
  | ([ range(1;($e|length)) as $i | ($e[$i].prev == $e[$i-1].hash) ] | all) as $chained
  | ([ $t[] | select(.payload=="EDITED") ] | length > 0) as $hastamper
  | {version:"patchline.merkle-audit-log/v1",
     entries:($e|length), chained:$chained,
     genesis:($e[0].prev=="GENESIS"),
     tamper_present:$hastamper}

' "$SPEC" > "$OUT/out.json"
{ echo "# Merkle-chained audit log"; echo; echo "Entries $(jq -r .entries "$OUT/out.json"); chained $(jq -r .chained "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "merkle-audit-log worker: chained=$(jq -r .chained "$OUT/out.json")"
