#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/citation-doi-gate.json}"; OUT="${2:-results/generated/citation-doi}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.citation-doi-gate/v1"' "$SPEC" > /dev/null
jq '

  .doi as $d | .bad_doi as $bd
  | ($d|test("^10\\.[0-9]+/.+")) as $valid
  | (((.title|length)>0) and ((.authors|length)>0) and ((.cite_version|length)>0) and (.year!=null)) as $complete
  | {version:"patchline.citation-doi/v1",
     doi:$d, doi_valid:$valid, complete:$complete,
     version_matches:(.cite_version==.release_version),
     bad_doi_valid:($bd|test("^10\\.[0-9]+/.+"))}

' "$SPEC" > "$OUT/out.json"
{ echo "# Citation file and archival DOI"; echo; echo "DOI $(jq -r .doi "$OUT/out.json"); valid $(jq -r .doi_valid "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "citation-doi worker: doi_valid=$(jq -r .doi_valid "$OUT/out.json") complete=$(jq -r .complete "$OUT/out.json")"
