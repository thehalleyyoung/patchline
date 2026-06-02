#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/canonical-json-gate.json}"
OUT="${2:-results/generated/canonical-json}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.canonical-json-gate/v1" and (.doc_a|type=="object")' "$SPEC" > /dev/null

# Canonical form: recursively sort object keys, compact separators (jq -cS).
canon() { jq -cS "$1" "$SPEC"; }
checksum() { canon "$1" | shasum -a 256 | cut -d' ' -f1; }

ca="$(canon '.doc_a')"; cb="$(canon '.doc_b')"; cc="$(canon '.doc_c')"
ha="$(checksum '.doc_a')"; hb="$(checksum '.doc_b')"; hc="$(checksum '.doc_c')"

jq -n \
  --arg ca "$ca" --arg cb "$cb" --arg cc "$cc" \
  --arg ha "$ha" --arg hb "$hb" --arg hc "$hc" '{
  version: "patchline.canonical-json/v1",
  canonical: { a: $ca, b: $cb, c: $cc },
  checksum: { a: $ha, b: $hb, c: $hc },
  reordered_collides: ($ca == $cb and $ha == $hb),
  content_change_differs: ($ha != $hc)
}' > "$OUT/canonical-json.json"

{
  echo "# Canonical JSON + checksum"
  echo
  echo "- doc_a checksum: \`$ha\`"
  echo "- doc_b checksum (reordered twin of a): \`$hb\`"
  echo "- doc_c checksum (content changed): \`$hc\`"
  echo
  echo "Reordered twin collides with a: $([ "$ha" = "$hb" ] && echo true || echo false)"
  echo
  echo "Content change differs from a: $([ "$ha" != "$hc" ] && echo true || echo false)"
} > "$OUT/canonical-json.md"
cp "$OUT/canonical-json.md" "$OUT/README.md"

echo "canonical-json worker: a==b? $([ "$ha" = "$hb" ] && echo yes || echo no); a!=c? $([ "$ha" != "$hc" ] && echo yes || echo no)"
