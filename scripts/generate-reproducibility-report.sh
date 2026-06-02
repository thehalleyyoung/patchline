#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/reproducibility-report-gate.json}"
OUT="${2:-results/generated/reproducibility-report}"
rm -rf "$OUT"
mkdir -p "$OUT/gates"

jq -e '
  .version == "patchline.reproducibility-report-gate/v1" and
  (.claim | length) > 150 and
  (.report_month | test("^[0-9]{4}-[0-9]{2}$")) and
  (.gates | length) >= .minimum_gates and
  (.required_sections | length) >= 5 and
  all(.gates[];
    (.id | test("^[a-z0-9-]+$")) and
    (.target | startswith("make ")) and
    (.script | startswith("scripts/")) and
    (.spec | startswith("examples/")) and
    (.summary | length) > 0 and
    (.public_repos | length) > 0 and
    (.trend_metric | length) > 0 and
    (.previous_value | type) == "number" and
    (.expected_fix | length) > 50
  )
' "$SPEC" > /dev/null

month="$(jq -r '.report_month' "$SPEC")"
gate_rows=()
gate_count="$(jq '.gates | length' "$SPEC")"
for ((i=0; i<gate_count; i++)); do
  id="$(jq -r ".gates[$i].id" "$SPEC")"
  target="$(jq -r ".gates[$i].target" "$SPEC")"
  script="$(jq -r ".gates[$i].script" "$SPEC")"
  gate_spec="$(jq -r ".gates[$i].spec" "$SPEC")"
  summary_rel="$(jq -r ".gates[$i].summary" "$SPEC")"
  trend_metric="$(jq -r ".gates[$i].trend_metric" "$SPEC")"
  previous_value="$(jq ".gates[$i].previous_value" "$SPEC")"
  expected_fix="$(jq -r ".gates[$i].expected_fix" "$SPEC")"
  gate_out="$OUT/gates/$id"
  run_log="$OUT/gates/$id.run.log"
  mkdir -p "$gate_out"
  started="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  status="passed"
  exit_code=0
  set +e
  bash "$script" "$gate_spec" "$gate_out" > "$run_log" 2>&1
  exit_code=$?
  set -e
  finished="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  if [[ "$exit_code" != "0" ]]; then
    status="failed"
  fi
  summary_path="$gate_out/$summary_rel"
  current_value=0
  if [[ -s "$summary_path" ]]; then
    current_value="$(jq --arg metric "$trend_metric" '(.[$metric] // .summary[$metric] // .metrics[$metric] // 0)' "$summary_path")"
  fi
  cache_files=0
  cache_bytes=0
  if [[ -d "$gate_out/cache" ]]; then
    cache_files="$(find "$gate_out/cache" -type f | wc -l | tr -d ' ')"
    cache_bytes="$(find "$gate_out/cache" -type f -print0 | xargs -0 stat -f %z 2>/dev/null | awk '{sum += $1} END {print sum + 0}')"
  fi
  if [[ "$cache_files" == "0" ]]; then
    nested_cache="$(find "$gate_out" -type d -name cache | head -n 1 || true)"
    if [[ -n "$nested_cache" ]]; then
      cache_files="$(find "$nested_cache" -type f | wc -l | tr -d ' ')"
      cache_bytes="$(find "$nested_cache" -type f -print0 | xargs -0 stat -f %z 2>/dev/null | awk '{sum += $1} END {print sum + 0}')"
    fi
  fi
  jq -n \
    --arg id "$id" \
    --arg target "$target" \
    --arg script "$script" \
    --arg spec "$gate_spec" \
    --arg status "$status" \
    --arg started "$started" \
    --arg finished "$finished" \
    --arg summary_path "$summary_path" \
    --arg run_log "$run_log" \
    --arg trend_metric "$trend_metric" \
    --arg expected_fix "$expected_fix" \
    --argjson exit_code "$exit_code" \
    --argjson previous_value "$previous_value" \
    --argjson current_value "$current_value" \
    --argjson cache_files "$cache_files" \
    --argjson cache_bytes "$cache_bytes" \
    --argjson public_repos "$(jq ".gates[$i].public_repos" "$SPEC")" \
    '{
      id:$id,
      target:$target,
      script:$script,
      spec:$spec,
      status:$status,
      exit_code:$exit_code,
      started_at:$started,
      finished_at:$finished,
      public_repos:$public_repos,
      summary_path:$summary_path,
      cache:{files:$cache_files, bytes:$cache_bytes, status:(if $cache_files > 0 then "populated" else "no-cache-files" end)},
      run_log:$run_log,
      failure:(if $status == "failed" then {log:$run_log, exit_code:$exit_code, expected_fix:$expected_fix} else null end),
      fix:(if $status == "passed" then "No fix required; regenerated public gate matched current expectations." else $expected_fix end),
      trend:{metric:$trend_metric, previous:$previous_value, current:$current_value, delta:($current_value - $previous_value)},
      verified:($status == "passed")
    }' > "$gate_out/report-row.json"
  gate_rows+=("$gate_out/report-row.json")
done

jq -s \
  --slurpfile spec "$SPEC" \
  --arg month "$month" \
  '{
    version:"patchline.reproducibility-report/v1",
    month:$month,
    claim:$spec[0].claim,
    gates:.,
    summary:{
      gates:length,
      passed:([.[] | select(.status == "passed")] | length),
      failed:([.[] | select(.status == "failed")] | length),
      public_repos:([.[].public_repos[]] | unique),
      cache_files:([.[].cache.files] | add),
      cache_bytes:([.[].cache.bytes] | add),
      trend_deltas:([.[] | {id, metric:.trend.metric, previous:.trend.previous, current:.trend.current, delta:.trend.delta}]),
      fixes:([.[] | {id, fix}]),
      verified:all(.[]; .verified == true)
    }
  }' "${gate_rows[@]}" > "$OUT/reproducibility-report.json"

{
  echo "# Patchline monthly reproducibility report: $month"
  echo
  echo "This report reruns public gates and records cache status, failures, fixes, and benchmark trends."
  echo
  echo "## Public gates"
  echo
  echo "| Gate | Status | Public repos | Trend | Cache status |"
  echo "| --- | --- | --- | --- | --- |"
  jq -r '.gates[] | "| `" + .target + "` | " + .status + " | " + (.public_repos | join(", ")) + " | " + .trend.metric + ": " + (.trend.previous|tostring) + " -> " + (.trend.current|tostring) + " (" + (.trend.delta|tostring) + ") | " + .cache.status + " / " + (.cache.files|tostring) + " files |"' "$OUT/reproducibility-report.json"
  echo
  echo "## Cache status"
  echo
  jq -r '.summary | "- cache files: `" + (.cache_files|tostring) + "`\n- cache bytes: `" + (.cache_bytes|tostring) + "`"' "$OUT/reproducibility-report.json"
  echo
  echo "## Failures"
  echo
  if [[ "$(jq '.summary.failed' "$OUT/reproducibility-report.json")" == "0" ]]; then
    echo "No public gate failures were observed."
  else
    jq -r '.gates[] | select(.status == "failed") | "- `" + .id + "` failed with exit code `" + (.exit_code|tostring) + "`; expected fix: " + .failure.expected_fix' "$OUT/reproducibility-report.json"
  fi
  echo
  echo "## Fixes"
  echo
  jq -r '.summary.fixes[] | "- `" + .id + "`: " + .fix' "$OUT/reproducibility-report.json"
  echo
  echo
  echo "## Benchmark trends"
  echo
  jq -r '.summary.trend_deltas[] | "- `" + .id + "` " + .metric + ": `" + (.previous|tostring) + "` -> `" + (.current|tostring) + "` (delta `" + (.delta|tostring) + "`)"' "$OUT/reproducibility-report.json"
} > "$OUT/report.md"

cp "$OUT/report.md" "$OUT/README.md"
echo "reproducibility report generated: gates $(jq '.summary.gates' "$OUT/reproducibility-report.json"), passed $(jq '.summary.passed' "$OUT/reproducibility-report.json"), cache files $(jq '.summary.cache_files' "$OUT/reproducibility-report.json")"
