#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
OUT="${1:-results/generated/db-version-semantics}"
SQL="examples/db-version-semantics/semantics.sql"
rm -rf "$OUT"; mkdir -p "$OUT"

go test ./internal/dbsemantics ./cmd/patchline -run 'Test(CatalogCoversStage66Engines|PostgresVersionSpecificDefaultSemantics|MySQLInstantAddColumnIsVersionSpecific|EngineNegativeControls|CloudAndAnalyticalEnginesHaveDistinctSemantics|RejectsUnsupportedEngineAndBadVersion|DBSemanticsCommand)' > "$OUT/go-test.log"

run_case() {
  local engine="$1" version="$2" name="$3"
  go run ./cmd/patchline db-semantics --engine "$engine" --version "$version" --sql "$SQL" --out "$OUT/$name.json" --json > "$OUT/$name.stdout.json"
}

run_case postgres 10 postgres10
run_case postgres 15 postgres15
run_case mysql 5.7 mysql57
run_case mysql 8.0.34 mysql80
run_case sqlite 3.45 sqlite345
run_case sqlserver 2022 sqlserver2022
run_case oracle 23 oracle23
run_case bigquery 2024.2 bigquery2024
run_case snowflake 8.20 snowflake820
run_case clickhouse 24.1 clickhouse241

jq -e '.profile.engine=="postgres" and any(.statements[].rules[]?; .id=="postgres.pre11_table_rewrite_default")' "$OUT/postgres10.json" > /dev/null
jq -e '.profile.engine=="postgres" and any(.statements[].rules[]?; .id=="postgres.v11_metadata_only_default")' "$OUT/postgres15.json" > /dev/null
jq -e '.summary.engine_negative_controls >= 2 and any(.statements[].engine_negative_controls[]?; .id=="postgres_pre11_default_rewrite" and .control_rule=="postgres.pre11_table_rewrite_default" and .control_risk=="high") and any(.statements[].engine_negative_controls[]?; .id=="postgres_pre82_concurrent_index_unsupported" and .control_rule=="postgres.pre82_concurrent_index_unsupported" and .control_verdict=="refuted")' "$OUT/postgres15.json" > /dev/null
jq -e 'any(.statements[].rules[]?; .id=="mysql.copy_or_preinstant_alter")' "$OUT/mysql57.json" > /dev/null
jq -e 'any(.statements[].rules[]?; .id=="mysql.v8_instant_add_column")' "$OUT/mysql80.json" > /dev/null
jq -e '.summary.engine_negative_controls >= 1 and any(.statements[].engine_negative_controls[]?; .id=="mysql_preinstant_copy_alter" and .control_rule=="mysql.copy_or_preinstant_alter" and .control_risk=="high")' "$OUT/mysql80.json" > /dev/null
jq -e 'any(.statements[].rules[]?; .id=="sqlite.foreign_keys_off")' "$OUT/sqlite345.json" > /dev/null
jq -e 'any(.statements[].rules[]?; .id=="sqlserver.offline_index_schema_lock")' "$OUT/sqlserver2022.json" > /dev/null
jq -e 'any(.statements[].rules[]?; .id=="sqlserver.online_index_lock_reduced") and .summary.engine_negative_controls >= 1 and any(.statements[].engine_negative_controls[]?; .id=="sqlserver_pre2012_online_index_schema_lock" and .control_rule=="sqlserver.offline_index_schema_lock" and .control_risk=="high")' "$OUT/sqlserver2022.json" > /dev/null
jq -e 'any(.statements[].rules[]?; .id=="oracle.modify_not_null_validates_rows" or .id=="engine.implicit_ddl_commit")' "$OUT/oracle23.json" > /dev/null
jq -e 'any(.statements[].rules[]?; .id=="bigquery.create_or_replace_replaces_table")' "$OUT/bigquery2024.json" > /dev/null
jq -e 'any(.statements[].rules[]?; .id=="snowflake.create_or_replace_swaps_identity")' "$OUT/snowflake820.json" > /dev/null
jq -e 'any(.statements[].rules[]?; .id=="clickhouse.async_mutation")' "$OUT/clickhouse241.json" > /dev/null

if go run ./cmd/patchline db-semantics --engine toydb --version 1 --sql "$SQL" --json > "$OUT/bad.stdout" 2> "$OUT/bad.stderr"; then
  echo "expected unsupported engine to fail" >&2
  exit 1
fi

jq -n \
  --slurpfile pg10 "$OUT/postgres10.json" \
  --slurpfile pg15 "$OUT/postgres15.json" \
  --slurpfile mysql57 "$OUT/mysql57.json" \
  --slurpfile mysql80 "$OUT/mysql80.json" \
  --slurpfile sqlite "$OUT/sqlite345.json" \
  --slurpfile sqlserver "$OUT/sqlserver2022.json" \
  --slurpfile oracle "$OUT/oracle23.json" \
  --slurpfile bigquery "$OUT/bigquery2024.json" \
  --slurpfile snowflake "$OUT/snowflake820.json" \
  --slurpfile clickhouse "$OUT/clickhouse241.json" \
  '{version:"patchline.db-version-semantics-gate/v1", ok:true, engines:["postgres","mysql","sqlite","sqlserver","oracle","bigquery","snowflake","clickhouse"], reports:[$pg10[0].hash,$pg15[0].hash,$mysql57[0].hash,$mysql80[0].hash,$sqlite[0].hash,$sqlserver[0].hash,$oracle[0].hash,$bigquery[0].hash,$snowflake[0].hash,$clickhouse[0].hash], engine_negative_controls:($pg15[0].summary.engine_negative_controls + $mysql80[0].summary.engine_negative_controls + $sqlserver[0].summary.engine_negative_controls), unsupported_engine_rejected:true}' \
  > "$OUT/gate-summary.json"

cat > "$OUT/README.md" <<'EOF'
# Database version semantics gate

This gate evaluates one real SQL fixture across PostgreSQL, MySQL, SQLite, SQL Server, Oracle, BigQuery, Snowflake, and ClickHouse profiles. It proves that version-specific rules change verdicts, that safe-looking or lower-impact migrations carry computed unsafe counter-profile controls, that cloud/analytical engines carry distinct replacement or async-mutation semantics, and that an unsupported engine is rejected.
EOF

echo "db-version-semantics gate passed: 8 engines, version-specific verdicts, engine negative controls, unsupported engine rejected"
