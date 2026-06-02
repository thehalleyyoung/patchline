#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/language-extractors-gate.json}"; OUT="${2:-results/generated/language-extractors}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.language-extractors-gate/v1" and (.findings|length) >= 1' "$SPEC" > /dev/null
jq '
  def valid($f; $req): ($req - ($f | keys)) == [];
  .required_fields as $req
  | {
      version: "patchline.language-extractors/v1",
      languages: ([ .findings[].language ] | unique),
      per_finding: [ .findings[] | {id, language, valid: valid(.; $req)} ],
      all_valid: ([ .findings[] | valid(.; $req) ] | all),
      malformed_valid: valid(.malformed_finding; $req),
      malformed_missing: ($req - (.malformed_finding | keys))
    }
' "$SPEC" > "$OUT/extractors.json"
{ echo "# Per-language finding schema"; echo; echo "Languages: $(jq -rc '.languages' "$OUT/extractors.json")"; echo "All valid: $(jq -r '.all_valid' "$OUT/extractors.json"); malformed valid: $(jq -r '.malformed_valid' "$OUT/extractors.json")"; } > "$OUT/extractors.md"
cp "$OUT/extractors.md" "$OUT/README.md"
echo "language-extractors worker: all_valid=$(jq -r '.all_valid' "$OUT/extractors.json") malformed_valid=$(jq -r '.malformed_valid' "$OUT/extractors.json")"
