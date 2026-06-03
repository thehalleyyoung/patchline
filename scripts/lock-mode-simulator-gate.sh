#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/lock-mode-simulator-gate.json}"
OUT="${2:-results/generated/lock-mode-simulator}"
rm -rf "$OUT"
mkdir -p "$OUT/reports"

jq -e '.version == "patchline.lock-mode-simulator-gate/v1" and (.claim|length) > 200 and (.cases|length) >= .minimum_engines' "$SPEC" > /dev/null

for phrase in "lock-mode simulator" "containerized smoke" "make lock-mode-simulator-gate"; do
  grep -F "$phrase" docs/lock-mode-simulator.md README.md > /dev/null
done

go test ./internal/dbsemantics ./cmd/patchline -run 'TestLockModeSimulator|TestDBSemanticsCommandWritesVersionedReport' -count=1 > "$OUT/go-test.log"

rows=()
while IFS= read -r encoded; do
  case_json="$(printf '%s' "$encoded" | base64 --decode)"
  id="$(jq -r '.id' <<<"$case_json")"
  engine="$(jq -r '.engine' <<<"$case_json")"
  version="$(jq -r '.version' <<<"$case_json")"
  sql="$(jq -r '.sql' <<<"$case_json")"
  report="$OUT/reports/$id.json"

  go run ./cmd/patchline db-semantics --engine "$engine" --version "$version" --sql "$sql" --out "$report" --json > "$OUT/reports/$id.stdout.json"

  jq -e \
    --arg mode "$(jq -r '.expect.mode' <<<"$case_json")" \
    --arg duration "$(jq -r '.expect.duration_class' <<<"$case_json")" \
    --argjson readers "$(jq -r '.expect.blocks_readers' <<<"$case_json")" \
    --argjson writers "$(jq -r '.expect.blocks_writers' <<<"$case_json")" \
    --argjson ddl "$(jq -r '.expect.blocks_ddl' <<<"$case_json")" \
    --argjson online "$(jq -r '.expect.online' <<<"$case_json")" \
    '.summary.lock_simulations == 1 and
     .summary.ddl_blocking_locks == 1 and
     .statements[0].lock_simulation.mode == $mode and
     .statements[0].lock_simulation.duration_class == $duration and
     .statements[0].lock_simulation.blocks_readers == $readers and
     .statements[0].lock_simulation.blocks_writers == $writers and
     .statements[0].lock_simulation.blocks_ddl == $ddl and
     .statements[0].lock_simulation.online == $online and
     (.statements[0].lock_simulation.conflicts|length) == 3 and
     (.statements[0].lock_simulation.documented_behavior|length) >= 1 and
     (.statements[0].lock_simulation.container_smoke_test.image|length) > 0' \
    "$report" > /dev/null

  jq -n \
    --arg id "$id" \
    --arg engine "$engine" \
    --slurpfile report "$report" \
    '{id:$id, engine:$engine, mode:$report[0].statements[0].lock_simulation.mode, duration_class:$report[0].statements[0].lock_simulation.duration_class, blocks_writers:$report[0].statements[0].lock_simulation.blocks_writers, documented_behavior:($report[0].statements[0].lock_simulation.documented_behavior|length), container_smoke:$report[0].statements[0].lock_simulation.container_smoke_test, verified:true}' > "$OUT/reports/$id.row.json"
  rows+=("$OUT/reports/$id.row.json")
done < <(jq -r '.cases[] | @base64' "$SPEC")

container_status="skipped-set-PATCHLINE_RUN_CONTAINERS"
container_runtime=""
if [[ "${PATCHLINE_RUN_CONTAINERS:-0}" == "1" ]]; then
  if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
    container_runtime="docker"
  elif command -v podman >/dev/null 2>&1 && podman info >/dev/null 2>&1; then
    container_runtime="podman"
  fi
  if [[ -n "$container_runtime" ]]; then
    "$container_runtime" run --rm -v "$ROOT:/work" -w /work "$(jq -r '.container_image' "$SPEC")" bash -lc "$(jq -r '.container_command' "$SPEC")" > "$OUT/container-smoke.log"
    container_status="passed"
  else
    container_status="skipped-container-runtime-unavailable"
  fi
fi

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  --arg container_status "$container_status" \
  --arg container_runtime "$container_runtime" \
  '{
    version:"patchline.lock-mode-simulator-results/v1",
    claim:$spec[0].claim,
    cases:$rows[0],
    summary:{
      engines:($rows[0] | map(.engine) | unique | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      writer_blocking_modes:($rows[0] | map(select(.blocks_writers == true)) | length),
      container_status:$container_status,
      container_runtime:$container_runtime
    }
  }' > "$OUT/lock-mode-simulator.json"

jq -e --slurpfile spec "$SPEC" '
  .summary.engines >= $spec[0].minimum_engines and
  .summary.verified == (.cases|length) and
  .summary.writer_blocking_modes >= 2 and
  (.cases | all(.documented_behavior >= 1 and (.container_smoke.image|length) > 0))
' "$OUT/lock-mode-simulator.json" > /dev/null

{
  echo "# Lock-mode simulator"
  echo
  echo "Container smoke status: $(jq -r '.summary.container_status' "$OUT/lock-mode-simulator.json")"
  echo
  echo "| Engine | Mode | Duration | Blocks writers |"
  echo "|---|---|---|---:|"
  jq -r '.cases[] | "| \(.engine) | \(.mode) | \(.duration_class) | \(.blocks_writers) |"' "$OUT/lock-mode-simulator.json"
} > "$OUT/lock-mode-simulator.md"
cp "$OUT/lock-mode-simulator.md" "$OUT/README.md"

echo "lock-mode simulator gate passed: $(jq -r '.summary.engines' "$OUT/lock-mode-simulator.json") engines, container_smoke=$(jq -r '.summary.container_status' "$OUT/lock-mode-simulator.json")"
