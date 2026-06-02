#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/db-dry-run-gate.json}"
OUT="${2:-results/generated/db-dry-run-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '.version == "patchline.db-dry-run-gate/v1" and .minimum_public_repos >= 4 and (.required_dialects | length) == 2' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref path; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  gh api "repos/$repo/contents/$path?ref=$ref" --jq '.content' | base64 --decode > "$case_out/migration.sql"
  go run ./cmd/patchline analyze-migration "$case_out/migration.sql" --json > "$case_out/analysis.json"
  jq -e '.statements | map(select((.kind == "update" or .kind == "delete") and (.table // "") != "")) | length > 0' "$case_out/analysis.json" > /dev/null
  jq -n \
    --arg name "db-dry-run-$id" \
    --arg incident "public-$id" \
    --slurpfile analysis "$case_out/analysis.json" \
    '($analysis[0].statements | map(select((.kind == "update" or .kind == "delete") and (.table // "") != ""))[0]) as $stmt |
    {
      version:"patchline.repair/v1",
      name:$name,
      incident:$incident,
      scope:{table:$stmt.table, where:{id:"patchline_schema_only"}},
      preconditions:[{kind:"sql", expr:"select 1", expect:"1"}],
      operations:[
        if $stmt.kind == "delete" then
          {id:"schema-only-delete", kind:"delete", table:$stmt.table, where:{id:"patchline_schema_only"}}
        else
          {id:"schema-only-update", kind:"update", table:$stmt.table, where:{id:"patchline_schema_only"}, set:{patchline_dry_run_marker:$incident}}
        end
      ],
      postconditions:[{kind:"sql", expr:"select 1", expect:"1"}],
      rollback:{strategy:"snapshot", snapshot_required:true}
    }' > "$case_out/manifest.json"
  for dialect in postgres mysql; do
    go run ./cmd/patchline db-dry-run "$case_out/manifest.json" --dialect "$dialect" --json > "$case_out/$dialect.json"
    jq -e '
      .ok == true and
      .mode == "schema-only" and
      (.credential_policy | contains("production credentials")) and
      .container.no_production_credentials == true and
      (.script | contains("EXPLAIN")) and
      (.script | contains("ROLLBACK")) and
      (.statements | length) > 0 and
      (.schema | length) > 0
    ' "$case_out/$dialect.json" > /dev/null
  done
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg ref "$ref" \
    --arg path "$path" \
    --slurpfile analysis "$case_out/analysis.json" \
    --slurpfile manifest "$case_out/manifest.json" \
    --slurpfile postgres "$case_out/postgres.json" \
    --slurpfile mysql "$case_out/mysql.json" \
    '{
      id:$id,
      repo:$repo,
      ref:$ref,
      path:$path,
      source_statements:$analysis[0].summary.total_statements,
      table:$manifest[0].scope.table,
      operation:$manifest[0].operations[0].kind,
      dialects:["postgres", "mysql"],
      postgres_hash:$postgres[0].hash,
      mysql_hash:$mysql[0].hash,
      schema_only:($postgres[0].mode == "schema-only" and $mysql[0].mode == "schema-only"),
      no_production_credentials:($postgres[0].container.no_production_credentials and $mysql[0].container.no_production_credentials),
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .path] | @tsv' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.db-dry-run-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_repos:($rows[0] | map(.repo) | unique | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      dialects:($rows[0] | map(.dialects[]) | unique),
      schema_only:($rows[0] | map(select(.schema_only == true)) | length),
      no_production_credentials:($rows[0] | map(select(.no_production_credentials == true)) | length),
      update_hooks:($rows[0] | map(select(.operation == "update")) | length),
      delete_hooks:($rows[0] | map(select(.operation == "delete")) | length)
    }
  }' > "$OUT/db-dry-run-gate.json"

jq -e --slurpfile spec "$SPEC" '
  .summary.public_repos >= $spec[0].minimum_public_repos and
  .summary.verified == (.slices | length) and
  ((($spec[0].required_dialects - .summary.dialects) | length) == 0) and
  .summary.schema_only == (.slices | length) and
  .summary.no_production_credentials == (.slices | length) and
  .summary.update_hooks > 0 and
  .summary.delete_hooks > 0
' "$OUT/db-dry-run-gate.json" > /dev/null

echo "DB dry-run gate passed: $(jq '.summary.public_repos' "$OUT/db-dry-run-gate.json") public repos, dialects=$(jq -r '.summary.dialects | join("+")' "$OUT/db-dry-run-gate.json"), update_hooks=$(jq '.summary.update_hooks' "$OUT/db-dry-run-gate.json"), delete_hooks=$(jq '.summary.delete_hooks' "$OUT/db-dry-run-gate.json")"
