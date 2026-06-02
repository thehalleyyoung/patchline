#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/fixture-minimizer-gate.json}"
OUT="${2:-results/generated/fixture-minimizer}"
rm -rf "$OUT"
mkdir -p "$OUT/work" "$OUT/minimized" "$OUT/seeds"

BIN="$OUT/bin/patchline"
mkdir -p "$OUT/bin"
go build -o "$BIN" ./cmd/patchline

# predicate: does the analyzer produce a fact matching $2 when $1 (a file) is analyzed?
WORK="$OUT/work/case"
predicate() {
  local file="$1" pattern="$2" ext="$3"
  rm -rf "$WORK"
  mkdir -p "$WORK/src"
  cp "$file" "$WORK/src/fixture.$ext"
  rm -rf "$WORK/out"
  "$BIN" repo analyze "$WORK/src" --stages inventory --no-llm --out "$WORK/out" >/dev/null 2>&1 || return 1
  grep -Eq "$pattern" "$WORK/out/inventory/facts.jsonl" 2>/dev/null
}

# greedy 1-minimizer: removes any line that is not needed to keep the predicate true.
minimize() {
  local seed="$1" pattern="$2" ext="$3" out="$4"
  cp "$seed" "$out"
  if ! predicate "$out" "$pattern" "$ext"; then
    echo "seed does not satisfy predicate: $seed" >&2
    return 1
  fi
  local changed=1 i total cand
  while [ "$changed" -eq 1 ]; do
    changed=0
    total="$(wc -l < "$out" | tr -d ' ')"
    i=1
    while [ "$i" -le "$total" ]; do
      cand="$out.cand"
      sed "${i}d" "$out" > "$cand"
      if predicate "$cand" "$pattern" "$ext"; then
        mv "$cand" "$out"
        changed=1
        total="$(wc -l < "$out" | tr -d ' ')"
      else
        rm -f "$cand"
        i=$((i+1))
      fi
    done
  done
}

# verify 1-minimality: removing any single remaining line must break the predicate.
verify_one_minimal() {
  local file="$1" pattern="$2" ext="$3"
  local total i cand
  total="$(wc -l < "$file" | tr -d ' ')"
  i=1
  while [ "$i" -le "$total" ]; do
    cand="$file.vcand"
    sed "${i}d" "$file" > "$cand"
    if predicate "$cand" "$pattern" "$ext"; then
      rm -f "$cand"
      echo "not 1-minimal: line $i is removable" >&2
      return 1
    fi
    rm -f "$cand"
    i=$((i+1))
  done
  return 0
}

# --- Seed the real-repo Cassandra case by fetching a real migration with DROP TABLE. ---
rm -rf "$OUT/fetch"
"$BIN" repo fetch laaksomavrick/twitter-go --subpath migrations \
  --download-dir "$OUT/fetch/cache" --out "$OUT/fetch/out" >/dev/null 2>&1
real_cql="$(find "$OUT/fetch" -name '0003-tweets.cql' | head -1)"
test -s "$real_cql"
cp "$real_cql" "$OUT/seeds/cassandra.cql"

# --- Property fixtures for the other ecosystems (each contains the target plus noise). ---
cat > "$OUT/seeds/sql.sql" <<'SQL'
-- add audit columns
ALTER TABLE users ADD COLUMN last_seen timestamp;
CREATE INDEX idx_users_email ON users(email);
-- the destructive change we want to isolate
DROP TABLE legacy_sessions;
UPDATE users SET active = true WHERE active IS NULL;
COMMENT ON TABLE users IS 'core table';
SQL

cat > "$OUT/seeds/spark.py" <<'PY'
from pyspark.sql import SparkSession
spark = SparkSession.builder.getOrCreate()
df = spark.read.parquet("/in")
df = df.filter(df.valid == True)
df.write.mode("overwrite").saveAsTable("warehouse.orders")
print("done")
PY

cat > "$OUT/seeds/avro.avsc" <<'AVRO'
{
  "type": "record",
  "name": "Event",
  "namespace": "com.example",
  "fields": [
    {"name": "id", "type": "string", "default": ""},
    {"name": "ts", "type": "long"}
  ]
}
AVRO

# ecosystem | seed | fact-pattern | ext
cases=(
  "cassandra|$OUT/seeds/cassandra.cql|\"kind\":\"nosql_change\".*cassandra.*\"destructive\":\"true\"|cql"
  "sql|$OUT/seeds/sql.sql|\"kind\":\"schema_evolution\".*\"operation\":\"drop_table\"|sql"
  "spark|$OUT/seeds/spark.py|\"kind\":\"data_pipeline_change\".*spark.*\"destructive\":\"true\"|py"
  "avro|$OUT/seeds/avro.avsc|\"kind\":\"schema_compatibility\".*\"breaking\":\"true\"|avsc"
)

results="[]"
for c in "${cases[@]}"; do
  IFS='|' read -r name seed pattern ext <<< "$c"
  seed_lines="$(wc -l < "$seed" | tr -d ' ')"
  mout="$OUT/minimized/$name.$ext"
  if minimize "$seed" "$pattern" "$ext" "$mout"; then
    min_lines="$(wc -l < "$mout" | tr -d ' ')"
    one_minimal=false
    if verify_one_minimal "$mout" "$pattern" "$ext"; then one_minimal=true; fi
    results="$(jq -n --argjson r "$results" --arg name "$name" \
      --argjson seed_lines "$seed_lines" --argjson min_lines "$min_lines" \
      --argjson one_minimal "$one_minimal" \
      '$r + [{ecosystem:$name, seed_lines:$seed_lines, minimized_lines:$min_lines, one_minimal:$one_minimal, reduced: ($seed_lines > $min_lines)}]')"
  else
    results="$(jq -n --argjson r "$results" --arg name "$name" '$r + [{ecosystem:$name, error:"seed predicate failed"}]')"
  fi
done

ecosystems="$(jq 'length' <<< "$results")"
all_one_minimal="$(jq '[.[] | .one_minimal] | all' <<< "$results")"
all_reduced="$(jq '[.[] | .reduced] | all' <<< "$results")"
real_min_lines="$(jq '[.[] | select(.ecosystem=="cassandra") | .minimized_lines] | first' <<< "$results")"
real_seed_lines="$(jq '[.[] | select(.ecosystem=="cassandra") | .seed_lines] | first' <<< "$results")"

jq -n \
  --argjson results "$results" \
  --argjson ecosystems "$ecosystems" \
  --argjson all_one_minimal "$all_one_minimal" \
  --argjson all_reduced "$all_reduced" \
  --argjson real_seed_lines "$real_seed_lines" \
  --argjson real_min_lines "$real_min_lines" '
  {
    version: "patchline.fixture-minimizer/v1",
    ecosystems: $ecosystems,
    per_ecosystem: $results,
    all_one_minimal: $all_one_minimal,
    all_reduced: $all_reduced,
    real_repo: "laaksomavrick/twitter-go",
    real_seed_lines: $real_seed_lines,
    real_minimized_lines: $real_min_lines
  }
' > "$OUT/fixture-minimizer.json"

{
  echo "# Generated fixture minimizers"
  echo
  jq -r '"Patchline minimized destructive findings across `" + (.ecosystems|tostring) + "` ecosystems. The real `" + .real_repo + "` Cassandra migration was reduced from `" + (.real_seed_lines|tostring) + "` lines to a 1-minimal `" + (.real_minimized_lines|tostring) + "`-line fixture that still triggers the destructive change fact."' "$OUT/fixture-minimizer.json"
  echo
  echo "## Guarantees"
  jq -r '"- every ecosystem fixture was reduced below its seed size: `" + (.all_reduced|tostring) + "`\n- every minimized fixture is 1-minimal (removing any line drops the target): `" + (.all_one_minimal|tostring) + "`"' "$OUT/fixture-minimizer.json"
  echo
  echo "Each minimizer uses the real analyzer as an oracle and delta-debugs the fixture down to the smallest input that still reproduces a destructive data-change finding, giving contributors and bug reporters a minimal, deterministic reproduction for every supported ecosystem."
} > "$OUT/fixture-minimizer.md"
cp "$OUT/fixture-minimizer.md" "$OUT/README.md"

rm -rf "$OUT/work" "$OUT/fetch/cache"
echo "fixture minimizer complete: $ecosystems ecosystems, all 1-minimal=$all_one_minimal, real cassandra $real_seed_lines->$real_min_lines lines"
