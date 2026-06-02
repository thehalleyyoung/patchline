#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/citation-doi-gate.json}"; OUT="${2:-results/generated/citation-doi}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.citation-doi-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "DOI" "make citation-doi-gate"; do grep -F "$phrase" docs/citation-doi.md README.md > /dev/null; done
bash scripts/citation-doi.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.citation-doi/v1" and .doi_valid==true and .complete==true and .version_matches==true and .bad_doi_valid==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.citation-doi-gate-results/v1",doi_valid:$r[0].doi_valid,complete:$r[0].complete,bad_doi_rejected:($r[0].bad_doi_valid|not),verified:true}' > "$OUT/gate-summary.json"
echo "citation-doi gate passed: citation complete with valid DOI, malformed DOI rejected"
