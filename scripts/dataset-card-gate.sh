#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

CATALOG="${1:-examples/real-repo-catalog.json}"
OUT="${2:-results/generated/dataset-cards}"
rm -rf "$OUT"
mkdir -p "$OUT/cards"

license_for_repo() {
  repo="$1"
  if command -v gh >/dev/null 2>&1; then
    gh api "repos/$repo" --jq '.license.spdx_id // .license.name // "NOASSERTION"' 2>/dev/null || echo "NOASSERTION"
  else
    curl -L -sS "https://api.github.com/repos/$repo" | jq -r '.license.spdx_id // .license.name // "NOASSERTION"'
  fi
}

jq -e '.version == "patchline.real-repo-catalog/v1" and (.slices | length) >= 25' "$CATALOG" > /dev/null

rows=()
while IFS=$'\t' read -r id label source_host repo ref subpath ecosystem framework evidence; do
  card="$OUT/cards/$id.json"
  md="$OUT/cards/$id.md"
  license="$(license_for_repo "$repo")"
  fetch_command="go run ./cmd/patchline repo fetch $repo --ref $ref --subpath $subpath --out results/generated/repo/$id/fetch --json"
  analyze_command="go run ./cmd/patchline repo analyze --github $repo --ref $ref --subpath $subpath --stages inventory,baseline,propose,compare --no-llm --out results/generated/repo/$id/analysis --json"
  jq -n \
    --arg id "$id" \
    --arg label "$label" \
    --arg source_host "$source_host" \
    --arg repo "$repo" \
    --arg ref "$ref" \
    --arg subpath "$subpath" \
    --arg ecosystem "$ecosystem" \
    --arg framework "$framework" \
    --arg license "$license" \
    --arg fetch_command "$fetch_command" \
    --arg analyze_command "$analyze_command" \
    --argjson evidence "$evidence" \
    '{
      version: "patchline.dataset-card/v1",
      id: $id,
      label: $label,
      source_host: $source_host,
      repo: $repo,
      license: $license,
      commit: $ref,
      subpath: $subpath,
      ecosystem: $ecosystem,
      migration_framework: $framework,
      evidence_types: $evidence,
      known_limitations: [
        "slice-level analysis only; run the full repository for complete maintainer triage",
        "public source evidence only; private incidents, deploys, traces, and production logs are not included",
        "deterministic static analysis reports candidates and proof holes rather than claiming production causality"
      ],
      reproducibility_commands: [$fetch_command, $analyze_command]
    }' > "$card"
  {
    printf '# Dataset card: %s\n\n' "$label"
    printf -- '- repo: `%s`\n' "$repo"
    printf -- '- license: `%s`\n' "$license"
    printf -- '- commit: `%s`\n' "$ref"
    printf -- '- subpath: `%s`\n' "$subpath"
    printf -- '- ecosystem: `%s`\n' "$ecosystem"
    printf -- '- migration framework: `%s`\n' "$framework"
    printf '\n## Evidence types\n\n'
    jq -r '.evidence_types[] | "- " + .' "$card"
    printf '\n## Known limitations\n\n'
    jq -r '.known_limitations[] | "- " + .' "$card"
    printf '\n## Reproducibility commands\n\n'
    jq -r '.reproducibility_commands[] | "- `" + . + "`"' "$card"
  } > "$md"
  rows+=("$card")
done < <(jq -r '.slices[] | [.id, .label, .source_host, .repo, .ref, .subpath, .ecosystem, .migration_framework, (.available_evidence_types | tojson)] | @tsv' "$CATALOG")

jq -s '{
  version:"patchline.dataset-card-index/v1",
  card_count:length,
  licenses: ([.[].license] | unique),
  ecosystems: ([.[].ecosystem] | unique),
  frameworks: ([.[].migration_framework] | unique),
  cards: .
}' "${rows[@]}" > "$OUT/index.json"

{
  printf '# Patchline dataset cards\n\n'
  printf '| slice | license | commit | ecosystem | framework | card |\n'
  printf '| --- | --- | --- | --- | --- | --- |\n'
  jq -r '.cards[] | "| \(.id) | \(.license) | `\(.commit)` | \(.ecosystem) | \(.migration_framework) | [`md`](cards/\(.id).md) |"' "$OUT/index.json"
} > "$OUT/index.md"

jq -e '
  .card_count >= 25 and
  all(.cards[];
    .version == "patchline.dataset-card/v1" and
    (.license | length) > 0 and
    (.commit | test("^[0-9a-f]{40}$")) and
    (.ecosystem | length) > 0 and
    (.migration_framework | length) > 0 and
    (.evidence_types | length) > 0 and
    (.known_limitations | length) >= 3 and
    (.reproducibility_commands | length) >= 2
  )
' "$OUT/index.json" > /dev/null
test -s "$OUT/index.md"
echo "dataset-card gate passed: $(jq '.card_count' "$OUT/index.json") public slice cards generated"
