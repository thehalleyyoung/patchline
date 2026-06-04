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
  (.public_demo.summary.ranked_risks > 0) and
  (.command_results.failed == 0) and
  (.command_results.succeeded >= 10)
' "$SITE/site-manifest.json" > /dev/null

jq -e --slurpfile spec "$SPEC" '
  .version == "patchline.docs-command-results/v1" and
  .summary.failed == 0 and
  .summary.succeeded == .summary.total and
  (.commands | length) >= ($spec[0].command_samples | length) and
  (.commands as $commands | all($spec[0].command_samples[]; . as $id | any($commands[]; .id == $id and .success == true)))
' "$SITE/artifacts/command-results.json" > /dev/null

python3 - "$SITE" <<'PY'
import html.parser
import pathlib
import sys
import urllib.parse

site = pathlib.Path(sys.argv[1]).resolve()
missing = []

class LinkParser(html.parser.HTMLParser):
    def __init__(self):
        super().__init__()
        self.links = []

    def handle_starttag(self, tag, attrs):
        for key, value in attrs:
            if key in {"href", "src"} and value:
                self.links.append(value)

for html_file in site.rglob("*.html"):
    parser = LinkParser()
    parser.feed(html_file.read_text(encoding="utf-8"))
    for link in parser.links:
        parsed = urllib.parse.urlparse(link)
        if parsed.scheme or link.startswith("#") or link.startswith("mailto:"):
            continue
        target = (html_file.parent / parsed.path).resolve()
        if site not in target.parents and target != site:
            missing.append(f"{html_file.relative_to(site)} escapes site with {link}")
        elif not target.exists():
            missing.append(f"{html_file.relative_to(site)} -> {link}")

if missing:
    print("broken docs links:", file=sys.stderr)
    for item in missing:
        print(f"  {item}", file=sys.stderr)
    sys.exit(1)
PY

grep -F "Real public-repo output" "$SITE/index.html" > /dev/null
grep -F "Empirically shown in this repo" "$SITE/index.html" > /dev/null
grep -F "How Patchline decides what to call a risk" "$SITE/theory.html" > /dev/null
grep -F "finite effect lattice" "$SITE/theory.html" > /dev/null
grep -F "make theory-risk-paper-gate" "$SITE/theory.html" > /dev/null
grep -F "Forem, Bytebase, Mastodon, and Lobsters" "$SITE/scenarios/public-repositories.html" > /dev/null
grep -F "This is not just four repos" "$SITE/scenarios/public-repositories.html" > /dev/null
grep -F "Tested on many public repos" "$SITE/api/index.html" > /dev/null
grep -F "Public repo API manual" "$SITE/api/index.html" > /dev/null
grep -F "Intended to work with any GitHub repo; tested on many." "$SITE/api/index.html" > /dev/null
if grep -F "Empirical baseline for this manual" "$SITE/api/index.html" > /dev/null; then
  echo "api/index.html still contains removed empirical baseline section" >&2
  exit 1
fi
grep -F "Five Lobsters ranked risks worth review" "$SITE/api/index.html" > /dev/null
for risk in risk:970ac4cfcd161507 risk:2ded136b72a75f38 risk:4bf8b05aa283adf3 risk:93bcbce17738750e risk:a2044c7b196a722c; do
  grep -F "$risk" "$SITE/api/index.html" > /dev/null
  grep -F "$risk" "$OUT/analysis/baseline/baseline.json" > /dev/null
done
grep -F "repo analyze" "$SITE/api/analyze.html" > /dev/null
grep -F "repo fetch" "$SITE/api/fetch.html" > /dev/null
grep -F "repo offline" "$SITE/api/offline-redaction-ci.html" > /dev/null
grep -F "proposal.json" "$SITE/api/outputs.html" > /dev/null
grep -F "Maintainer tutorial" "$SITE/tutorials/maintainers.html" > /dev/null
grep -F "Researcher tutorial" "$SITE/tutorials/researchers.html" > /dev/null
grep -F "Security reviewer tutorial" "$SITE/tutorials/security-reviewers.html" > /dev/null
grep -F "Security review workflow" "$SITE/tutorials/security-reviewers.html" > /dev/null
grep -F "What to approve, reject, or ask for" "$SITE/tutorials/security-reviewers.html" > /dev/null
grep -F "Contributor tutorial" "$SITE/tutorials/contributors.html" > /dev/null
test -s "$SITE/artifacts/public-evidence.json"
test -s "$SITE/artifacts/public-repo-many-matrix-results.json"
jq -e '
  .version == "patchline.docs-public-evidence/v1" and
  .slice_summary.repos >= 4 and
  .slice_summary.slices >= 4 and
  .many_repo_matrix.summary.slices >= 11 and
  .many_repo_matrix.summary.unique_repos >= 10 and
  .many_repo_matrix.summary.ecosystems >= 10 and
  .many_repo_matrix.summary.offline_passed == .many_repo_matrix.summary.slices and
  .many_repo_matrix.summary.all_compare_checks_passed == true and
  .benchmark_summary.public_migration_cases >= 5 and
  .benchmark_summary.public_incident_cases >= 3 and
  .benchmark_summary.all_expected_ok == true and
  (.confirmed_public_incident_ground_truth | length) >= 2
' "$SITE/artifacts/public-evidence.json" > /dev/null
jq -e '
  .version == "patchline.public-repo-many-matrix-results/v1" and
  .summary.slices >= 11 and
  .summary.unique_repos >= 10 and
  .summary.total_ranked_risks >= 1 and
  .summary.offline_passed == .summary.slices and
  all(.cases[]; .offline_ok == true and .compare_checks_failed == 0)
' "$SITE/artifacts/public-repo-many-matrix-results.json" > /dev/null
grep -F "What Patchline can find" "$SITE/reference/findings.html" > /dev/null
grep -F "Confirmed bug and incident classes" "$SITE/reference/findings.html" > /dev/null
grep -F "Confirmed bugs and real-repo risk findings" "$SITE/index.html" > /dev/null
grep -F "$(jq -r '.site_url' "$SPEC")" "$SITE/sitemap.xml" > /dev/null

jq -n \
  --slurpfile manifest "$SITE/site-manifest.json" \
  --slurpfile demo "$SITE/artifacts/public-demo.json" \
  '{
    version:"patchline.docs-site-gate-results/v1",
    pages:($manifest[0].pages | length),
    roles:($manifest[0].roles | length),
    commands:$manifest[0].command_results.succeeded,
    files_scanned:$demo[0].files_scanned,
    ranked_risks:$demo[0].ranked_risks,
    generated_files:$demo[0].generated_files,
    verified:true
  }' > "$OUT/summary.json"

echo "docs site gate passed: pages $(jq '.pages' "$OUT/summary.json"), roles $(jq '.roles' "$OUT/summary.json"), risks $(jq '.ranked_risks' "$OUT/summary.json")"
