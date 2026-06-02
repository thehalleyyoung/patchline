#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/migration-age-stratification-gate.json}"
OUT="${2:-results/generated/migration-age-stratification}"
rm -rf "$OUT"
mkdir -p "$OUT/analyses" "$OUT/cache"

jq -e '
  .version == "patchline.migration-age-stratification-gate/v1" and
  (.claim | length) > 100 and
  (.slices | length) >= .minimum_repositories and
  all(.slices[]; (.repo | contains("/")) and (.subpath | length) > 0 and (.ref | test("^[0-9a-f]{40}$")))
' "$SPEC" > /dev/null

mig_jsonl="$OUT/migrations.jsonl"
: > "$mig_jsonl"

slice_count="$(jq '.slices | length' "$SPEC")"
for ((s=0; s<slice_count; s++)); do
  repo="$(jq -r ".slices[$s].repo" "$SPEC")"
  subpath="$(jq -r ".slices[$s].subpath" "$SPEC")"
  ecosystem="$(jq -r ".slices[$s].ecosystem" "$SPEC")"
  framework="$(jq -r ".slices[$s].framework" "$SPEC")"
  ref="$(jq -r ".slices[$s].ref" "$SPEC")"
  id="$(printf '%s' "${repo//\//-}" | tr -c 'A-Za-z0-9_.-' '-')"
  analysis="$OUT/analyses/$id"
  mkdir -p "$analysis"
  go run ./cmd/patchline repo analyze \
    --github "$repo" \
    --ref "$ref" \
    --subpath "$subpath" \
    --download-dir "$OUT/cache" \
    --stages inventory,baseline \
    --no-llm \
    --out "$analysis" \
    --json > "$OUT/analyze-$id.json"

  facts="$analysis/inventory/facts.jsonl"
  baseline="$analysis/baseline/baseline.json"

  # Per-migration records joining the inventory file list with baseline ranked risks.
  jq -n \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --arg ecosystem "$ecosystem" \
    --arg framework "$framework" \
    --arg ref "$ref" \
    --slurpfile facts <(jq -c 'select(.kind=="file")' "$facts") \
    --slurpfile risks <(jq -c '.risks' "$baseline") \
    '
    ($facts | map(.path) | map(select((. | gsub(".*/";"")) | test("^[0-9]+_"))) | unique) as $migrations |
    (
      $risks[0]
      | map({path, kind, severity})
      | group_by(.path)
      | map({
          key: .[0].path,
          value: {
            risks: length,
            high: ([.[] | select(.severity=="high")] | length),
            dml: ([.[] | select(.kind | test("(^|:)(update|delete|insert)$"))] | length),
            ddl: ([.[] | select(.kind | test("create|add_column|drop|remove_column|alter|table"))] | length)
          }
        })
      | from_entries
    ) as $byPath |
    ($migrations
      | map({path:., key:((. | gsub(".*/";"")) | capture("^(?<n>[0-9]+)_").n)})
      | sort_by(.key)
    ) as $sorted |
    ($sorted | length) as $count |
    $sorted
    | to_entries
    | map(
        .key as $rank |
        .value as $m |
        ($byPath[$m.path] // {risks:0,high:0,dml:0,ddl:0}) as $r |
        {
          repo: $repo,
          ecosystem: $ecosystem,
          framework: $framework,
          ref: $ref,
          path: $m.path,
          age_key: $m.key,
          age_rank: $rank,
          age_band: (if $count <= 1 then "recent" elif $rank >= ($count / 2) then "recent" else "old" end),
          risks: $r.risks,
          high_severity: $r.high,
          dml_ops: $r.dml,
          ddl_ops: $r.ddl,
          change_type: (
            if $r.dml > 0 then "backfill-heavy"
            elif $r.ddl > 0 then "schema-only"
            else "schema-only" end
          )
        }
      )
    | .[]
    ' >> "$mig_jsonl"
done

# Aggregate strata across all repositories.
jq -s '
  . as $rows |
  def band($b): [$rows[] | select(.age_band == $b)];
  def type($t): [$rows[] | select(.change_type == $t)];
  def stratum($name; $sel):
    ($sel) as $g |
    {
      stratum: $name,
      migrations: ($g | length),
      total_risks: ([$g[].risks] | add // 0),
      high_severity: ([$g[].high_severity] | add // 0),
      risk_density: (if ($g | length) > 0 then (([$g[].risks] | add // 0) / ($g | length)) else 0 end)
    };
  {
    version: "patchline.migration-age-stratification/v1",
    strata: [
      stratum("recent"; [$rows[] | select(.age_band=="recent")]),
      stratum("old"; [$rows[] | select(.age_band=="old")]),
      stratum("schema-only"; [$rows[] | select(.change_type=="schema-only")]),
      stratum("backfill-heavy"; [$rows[] | select(.change_type=="backfill-heavy")])
    ],
    cross_tab: [
      stratum("recent/schema-only"; [$rows[] | select(.age_band=="recent" and .change_type=="schema-only")]),
      stratum("recent/backfill-heavy"; [$rows[] | select(.age_band=="recent" and .change_type=="backfill-heavy")]),
      stratum("old/schema-only"; [$rows[] | select(.age_band=="old" and .change_type=="schema-only")]),
      stratum("old/backfill-heavy"; [$rows[] | select(.age_band=="old" and .change_type=="backfill-heavy")])
    ],
    by_repository: (
      $rows | group_by(.repo) | map({
        repo: .[0].repo,
        ecosystem: .[0].ecosystem,
        framework: .[0].framework,
        migrations: length,
        recent: ([.[] | select(.age_band=="recent")] | length),
        old: ([.[] | select(.age_band=="old")] | length),
        schema_only: ([.[] | select(.change_type=="schema-only")] | length),
        backfill_heavy: ([.[] | select(.change_type=="backfill-heavy")] | length),
        total_risks: ([.[].risks] | add // 0)
      })
    ),
    summary: {
      repositories: ($rows | map(.repo) | unique | length),
      migrations: ($rows | length),
      total_risks: ([$rows[].risks] | add // 0),
      backfill_heavy: ([$rows[] | select(.change_type=="backfill-heavy")] | length),
      schema_only: ([$rows[] | select(.change_type=="schema-only")] | length),
      strata_populated: (
        [ ([$rows[] | select(.age_band=="recent")] | length),
          ([$rows[] | select(.age_band=="old")] | length),
          ([$rows[] | select(.change_type=="schema-only")] | length),
          ([$rows[] | select(.change_type=="backfill-heavy")] | length)
        ] | map(select(. > 0)) | length
      ),
      verified: (($rows | length) > 0)
    }
  }
' "$mig_jsonl" > "$OUT/migration-age-stratification.json"

{
  echo "# Migration-age stratification"
  echo
  echo "Patchline stratifies real public migrations by age band (recent vs old, ranked by the migration's own ordinal/timestamp prefix within each repository) and by change type (schema-only DDL vs backfill-heavy data writes), then reports how ranked-risk density differs across strata."
  echo
  echo "## Summary"
  jq -r '.summary | "- repositories: `" + (.repositories|tostring) + "`\n- migrations: `" + (.migrations|tostring) + "`\n- total ranked risks: `" + (.total_risks|tostring) + "`\n- schema-only migrations: `" + (.schema_only|tostring) + "`\n- backfill-heavy migrations: `" + (.backfill_heavy|tostring) + "`"' "$OUT/migration-age-stratification.json"
  echo
  echo "## Strata"
  echo
  echo "| Stratum | Migrations | Total risks | High severity | Risk density |"
  echo "| --- | ---: | ---: | ---: | ---: |"
  jq -r '.strata[] | "| " + .stratum + " | " + (.migrations|tostring) + " | " + (.total_risks|tostring) + " | " + (.high_severity|tostring) + " | " + ((.risk_density*100|round/100)|tostring) + " |"' "$OUT/migration-age-stratification.json"
  echo
  echo "## Age x change-type cross-tab"
  echo
  echo "| Stratum | Migrations | Total risks | Risk density |"
  echo "| --- | ---: | ---: | ---: |"
  jq -r '.cross_tab[] | "| " + .stratum + " | " + (.migrations|tostring) + " | " + (.total_risks|tostring) + " | " + ((.risk_density*100|round/100)|tostring) + " |"' "$OUT/migration-age-stratification.json"
  echo
  echo "## By repository"
  echo
  echo "| Repo | Ecosystem | Migrations | Recent | Old | Schema-only | Backfill-heavy |"
  echo "| --- | --- | ---: | ---: | ---: | ---: | ---: |"
  jq -r '.by_repository[] | "| `" + .repo + "` | " + .ecosystem + " | " + (.migrations|tostring) + " | " + (.recent|tostring) + " | " + (.old|tostring) + " | " + (.schema_only|tostring) + " | " + (.backfill_heavy|tostring) + " |"' "$OUT/migration-age-stratification.json"
} > "$OUT/migration-age-stratification.md"

cp "$OUT/migration-age-stratification.md" "$OUT/README.md"
echo "migration-age stratification complete: repos $(jq '.summary.repositories' "$OUT/migration-age-stratification.json"), migrations $(jq '.summary.migrations' "$OUT/migration-age-stratification.json"), strata populated $(jq '.summary.strata_populated' "$OUT/migration-age-stratification.json")"
