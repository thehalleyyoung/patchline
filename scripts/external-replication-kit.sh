#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/external-replication-kit-gate.json}"; OUT="${2:-results/generated/external-replication-kit}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.external-replication-kit-gate/v1"' "$SPEC" > /dev/null
jq '
  def r4: (.*10000|round)/10000;
  .tolerance as $tol | .claims as $C
  | ([ $C[] | select( (.expected - .recomputed | if . < 0 then -. else . end) <= $tol ) ]|length) as $ok
  | .tampered_claim as $t
  | {
      version: "patchline.external-replication-kit/v1",
      total_claims: ($C|length),
      reproduced: $ok,
      reproduced_fraction: (($ok / ($C|length)) | r4),
      all_reproduced: ($ok == ($C|length)),
      tamper_detected: ( ($t.expected - $t.recomputed | if . < 0 then -. else . end) > $tol )
    }
' "$SPEC" > "$OUT/kit.json"
{ echo "# External replication kit"; echo; echo "Reproduced: $(jq -r '.reproduced' "$OUT/kit.json")/$(jq -r '.total_claims' "$OUT/kit.json"); tamper detected: $(jq -r '.tamper_detected' "$OUT/kit.json")"; } > "$OUT/kit.md"
cp "$OUT/kit.md" "$OUT/README.md"
echo "external-replication-kit worker: all_reproduced=$(jq -r '.all_reproduced' "$OUT/kit.json") tamper_detected=$(jq -r '.tamper_detected' "$OUT/kit.json")"
