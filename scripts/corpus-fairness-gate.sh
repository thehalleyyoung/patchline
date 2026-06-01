#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

CATALOG="${1:-examples/real-repo-catalog.json}"
OUT="${2:-results/generated/corpus-fairness}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.real-repo-catalog/v1" and (.slices | length) >= 25' "$CATALOG" > /dev/null

jq '
  def counts($field): [.slices[] | .[$field]] | group_by(.) | map({name: .[0], count: length}) | sort_by(-.count, .name);
  (.slices | length) as $total |
  {
    version: "patchline.corpus-fairness/v1",
    total_slices: $total,
    ecosystem_coverage: counts("ecosystem"),
    framework_coverage: counts("migration_framework"),
    source_host_coverage: counts("source_host"),
    monorepo_slices: ([.slices[] | select(.monorepo == true)] | length),
    flags: (
      ([counts("ecosystem")[] | select((.count / $total) > 0.50) | {dimension:"ecosystem", value:.name, share:(.count / $total), reason:"one ecosystem is more than half the corpus"}]) +
      ([counts("migration_framework")[] | select((.count / $total) > 0.35) | {dimension:"migration_framework", value:.name, share:(.count / $total), reason:"one migration framework is more than 35% of the corpus"}]) +
      ([counts("source_host")[] | select((.count / $total) > 0.75) | {dimension:"source_host", value:.name, share:(.count / $total), reason:"one source host is more than 75% of the corpus"}])
    ),
    recommendations: [
      "Add more non-GitHub public sources to reduce source-host concentration.",
      "Keep per-ecosystem reporting enabled so aggregate results do not hide framework skew.",
      "Preserve monorepo and small-repo slices to avoid overfitting to one repository shape."
    ]
  }
' "$CATALOG" > "$OUT/fairness.json"

{
  printf '# Patchline corpus fairness audit\n\n'
  printf 'Total slices: %s\n\n' "$(jq '.total_slices' "$OUT/fairness.json")"
  printf '## Ecosystem coverage\n\n'
  jq -r '.ecosystem_coverage[] | "- \(.name): \(.count)"' "$OUT/fairness.json"
  printf '\n## Framework coverage\n\n'
  jq -r '.framework_coverage[] | "- \(.name): \(.count)"' "$OUT/fairness.json"
  printf '\n## Source-host coverage\n\n'
  jq -r '.source_host_coverage[] | "- \(.name): \(.count)"' "$OUT/fairness.json"
  printf '\n## Over-reliance flags\n\n'
  jq -r '.flags[]? | "- \(.dimension)=\(.value) share=\(.share): \(.reason)"' "$OUT/fairness.json"
  printf '\n## Recommendations\n\n'
  jq -r '.recommendations[] | "- " + .' "$OUT/fairness.json"
} > "$OUT/fairness.md"

jq -e '
  .version == "patchline.corpus-fairness/v1" and
  .total_slices >= 25 and
  (.ecosystem_coverage | length) >= 7 and
  (.framework_coverage | length) >= 12 and
  (.source_host_coverage | length) >= 1 and
  (.flags | length) >= 1 and
  (.recommendations | length) >= 3
' "$OUT/fairness.json" > /dev/null
test -s "$OUT/fairness.md"
echo "corpus fairness gate passed: $(jq '.total_slices' "$OUT/fairness.json") slices audited with $(jq '.flags | length' "$OUT/fairness.json") over-reliance flags"
