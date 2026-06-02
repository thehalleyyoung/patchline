#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/secret-leak-scanner-gate.json}"; OUT="${2:-results/generated/secret-leak-scanner}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.secret-leak-scanner-gate/v1"' "$SPEC" > /dev/null
jq '

  .patterns as $P | .artifacts as $A | .leaky_artifact as $L
  | ([ $A[] | .content as $c | ($P[] | select(. as $p | $c|test($p))) ]|length) as $leaks
  | ([ $L.content as $c | ($P[] | select(. as $p | $c|test($p))) ]|length) as $leakyhits
  | {version:"patchline.secret-leak-scanner/v1",
     scanned:($A|length), leaks:$leaks, clean:($leaks==0),
     leaky_detected:($leakyhits>0)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Secret-leak scanner"; echo; echo "Scanned $(jq -r .scanned "$OUT/out.json"); leaks $(jq -r .leaks "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "secret-leak-scanner worker: clean=$(jq -r .clean "$OUT/out.json") leaky_detected=$(jq -r .leaky_detected "$OUT/out.json")"
