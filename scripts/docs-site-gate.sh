#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/docs-site-gate.json}"
OUT="${2:-results/generated/docs-site-gate}"
mkdir -p "$(dirname "$OUT")"

for phrase in "GitHub Pages" "maintainers" "researchers" "security reviewers" "contributors" "make docs-site-gate"; do
  grep -F "$phrase" docs/hosted-docs-site.md README.md > /dev/null
done
grep -F "actions/deploy-pages" .github/workflows/docs.yml > /dev/null

bash scripts/build-docs-site.sh "$SPEC" "$OUT" > "$OUT.run.log"
SITE="$OUT/site"

while read -r page; do
  if [[ "$page" == ".nojekyll" ]]; then
    test -e "$SITE/$page"
  else
    test -s "$SITE/$page"
  fi
done < <(jq -r '.required_pages[]' "$SPEC")

while read -r role; do
  grep -F "$role" "$SITE/site-manifest.json" > /dev/null
  test -s "$SITE/tutorials/$role.html"
done < <(jq -r '.required_roles[]' "$SPEC")

min_files="$(jq '.real_code.minimum_files_scanned' "$SPEC")"
min_risks="$(jq '.real_code.minimum_ranked_risks' "$SPEC")"
min_generated="$(jq '.real_code.minimum_generated_files' "$SPEC")"
jq -e --argjson min_files "$min_files" --argjson min_risks "$min_risks" --argjson min_generated "$min_generated" '
  .version == "patchline.docs-public-demo/v1" and
  .files_scanned >= $min_files and
  .ranked_risks >= $min_risks and
  .generated_files >= $min_generated and
  .deterministic_only == true
' "$SITE/artifacts/public-demo.json" > /dev/null

jq -e '
  .version == "patchline.docs-site/v1" and
  (.roles | length) == 4 and
  (.pages | index("index.html")) != null and
  (.pages | index("sitemap.xml")) != null and
  (.public_demo.summary.ranked_risks > 0)
' "$SITE/site-manifest.json" > /dev/null

grep -F "Real public-repo output" "$SITE/index.html" > /dev/null
grep -F "Maintainer tutorial" "$SITE/tutorials/maintainers.html" > /dev/null
grep -F "Researcher tutorial" "$SITE/tutorials/researchers.html" > /dev/null
grep -F "Security reviewer tutorial" "$SITE/tutorials/security-reviewers.html" > /dev/null
grep -F "Contributor tutorial" "$SITE/tutorials/contributors.html" > /dev/null
grep -F "$(jq -r '.site_url' "$SPEC")" "$SITE/sitemap.xml" > /dev/null

jq -n \
  --slurpfile manifest "$SITE/site-manifest.json" \
  --slurpfile demo "$SITE/artifacts/public-demo.json" \
  '{
    version:"patchline.docs-site-gate-results/v1",
    pages:($manifest[0].pages | length),
    roles:($manifest[0].roles | length),
    files_scanned:$demo[0].files_scanned,
    ranked_risks:$demo[0].ranked_risks,
    generated_files:$demo[0].generated_files,
    verified:true
  }' > "$OUT/summary.json"

echo "docs site gate passed: pages $(jq '.pages' "$OUT/summary.json"), roles $(jq '.roles' "$OUT/summary.json"), risks $(jq '.ranked_risks' "$OUT/summary.json")"
