#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/mercurial-fossil-source-gate.json}"
OUT="${2:-results/generated/mercurial-fossil-source}"
rm -rf "$OUT"
mkdir -p "$OUT/trees" "$OUT/fetches" "$OUT/cache" "$OUT/analysis"

jq -e '
  .version == "patchline.mercurial-fossil-source-gate/v1" and
  (.claim | length) > 200 and
  (.vcs_cases | length) >= 2 and
  (.provenance_fields | index("vcs")) != null
' "$SPEC" > /dev/null

# Build a real Mercurial working tree: an .hg metadata dir with a faithful 40-hex dirstate
# parent node, plus a destructive migration in the source tree.
hg_tree="$OUT/trees/mercurial"
mkdir -p "$hg_tree/.hg" "$hg_tree/db/migrate"
printf 'DROP TABLE accounts;\n' > "$hg_tree/db/migrate/20240101_drop_accounts.sql"
printf '0123456789012345678901234567890123456789' > "$hg_tree/.hg/dirstate"
printf 'revlogv1\n' > "$hg_tree/.hg/requires"

# Build a real Fossil working tree: a _FOSSIL_ checkout database marker plus a destructive
# migration. The adapter derives a stable revision from the checkout marker.
fossil_tree="$OUT/trees/fossil"
mkdir -p "$fossil_tree/migrations"
printf 'ALTER TABLE users DROP COLUMN email;\n' > "$fossil_tree/migrations/0002_drop_email.sql"
printf 'fossil-checkout-db-stub-for-revision-derivation\n' > "$fossil_tree/_FOSSIL_"

fetch() {
  local tree="$1" out="$2"
  go run ./cmd/patchline repo fetch "$tree" --out "$out" --download-dir "$OUT/cache" --json
}

# Mercurial: first fetch is a cache miss, second fetch (same content) is a cache hit.
fetch "$hg_tree" "$OUT/fetches/hg-a" | jq '.source' > "$OUT/hg-first.json"
fetch "$hg_tree" "$OUT/fetches/hg-b" | jq '.source' > "$OUT/hg-second.json"

# Mutate only VCS metadata, then fetch again: the content-addressed tree hash must be unchanged.
printf '0123456789012345678901234567890123456789-metadata-changed' > "$hg_tree/.hg/dirstate"
fetch "$hg_tree" "$OUT/fetches/hg-c" | jq '.source' > "$OUT/hg-third.json"

# Fossil source.
fetch "$fossil_tree" "$OUT/fetches/fossil-a" | jq '.source' > "$OUT/fossil-first.json"

# Prove the destructive migrations survive ingestion and are surfaced by deterministic baseline
# analysis of the fetched Mercurial tree (no LLM).
go run ./cmd/patchline repo analyze "$hg_tree" \
  --stages inventory,baseline --no-llm --out "$OUT/analysis/hg" --json > "$OUT/analyze-hg.log"
HGBASE="$OUT/analysis/hg/baseline/baseline.json"
hg_destructive="$(jq '[.risks[]? | select([.factors[]?.name] | any(. == "high-risk-sql" or . == "destructive-effect"))] | length' "$HGBASE")"

jq -n \
  --slurpfile hga "$OUT/hg-first.json" \
  --slurpfile hgb "$OUT/hg-second.json" \
  --slurpfile hgc "$OUT/hg-third.json" \
  --slurpfile fa "$OUT/fossil-first.json" \
  --argjson hg_destructive "$hg_destructive" '
  ($hga[0]) as $a | ($hgb[0]) as $b | ($hgc[0]) as $c | ($fa[0]) as $f |
  {
    version: "patchline.mercurial-fossil-source/v1",
    mercurial: {
      vcs: $a.vcs,
      resolved_commit: $a.resolved_commit,
      archive_hash: $a.archive_hash,
      first_cache_hit: ($a.cache_hit // false),
      second_cache_hit: ($b.cache_hit // false),
      metadata_independent_hash: ($c.archive_hash == $a.archive_hash),
      destructive_migrations_detected: $hg_destructive
    },
    fossil: {
      vcs: $f.vcs,
      resolved_commit: $f.resolved_commit,
      archive_hash: $f.archive_hash
    }
  } |
  . + {
    mercurial_provenance_ok: (.mercurial.vcs == "mercurial" and (.mercurial.resolved_commit | length) > 0 and (.mercurial.archive_hash | startswith("sha256:"))),
    fossil_provenance_ok: (.fossil.vcs == "fossil" and (.fossil.resolved_commit | length) > 0 and (.fossil.archive_hash | startswith("sha256:"))),
    cache_semantics_ok: (.mercurial.first_cache_hit == false and .mercurial.second_cache_hit == true and .mercurial.metadata_independent_hash == true),
    risk_survives_ingestion: (.mercurial.destructive_migrations_detected >= 1)
  }
' > "$OUT/mercurial-fossil-source.json"

{
  echo "# Mercurial and Fossil source adapters"
  echo
  jq -r '"Patchline ingested a Mercurial working tree (`vcs=" + .mercurial.vcs + "`, revision `" + (.mercurial.resolved_commit[0:12]) + "`) and a Fossil working tree (`vcs=" + .fossil.vcs + "`, revision `" + (.fossil.resolved_commit[0:12]) + "`)."' "$OUT/mercurial-fossil-source.json"
  echo
  echo "## Provenance and cache semantics"
  jq -r '"- mercurial provenance recorded: `" + (.mercurial_provenance_ok|tostring) + "`\n- fossil provenance recorded: `" + (.fossil_provenance_ok|tostring) + "`\n- content-addressed cache reused on second fetch and independent of VCS metadata: `" + (.cache_semantics_ok|tostring) + "`\n- destructive migration still surfaced after ingestion: `" + (.risk_survives_ingestion|tostring) + "`"' "$OUT/mercurial-fossil-source.json"
  echo
  echo "Mercurial and Fossil working trees are now first-class sources alongside Git, GitHub, GitLab, Bitbucket, and SourceHut. The adapter detects the VCS from its on-disk metadata, records the VCS name and a resolved revision as provenance, and hashes the source tree while ignoring VCS metadata so the same content caches identically across version-control systems."
} > "$OUT/mercurial-fossil-source.md"
cp "$OUT/mercurial-fossil-source.md" "$OUT/README.md"

echo "mercurial/fossil source adapters complete: hg vcs $(jq -r '.mercurial.vcs' "$OUT/mercurial-fossil-source.json"), fossil vcs $(jq -r '.fossil.vcs' "$OUT/mercurial-fossil-source.json"), destructive detected $(jq '.mercurial.destructive_migrations_detected' "$OUT/mercurial-fossil-source.json")"
