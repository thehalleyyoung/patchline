#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/compatibility-gate.json}"
OUT="${2:-results/generated/compatibility-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/bin" "$OUT/cases" "$OUT/cache"

jq -e '
  .version == "patchline.compatibility-gate/v1" and
  (.claim | length) > 40 and
  (.minimal_tools | length) >= 5 and
  (.profiles | length) >= 2 and
  all(.profiles[]; (.id | length) > 0 and (.goos | test("^(darwin|linux)$")) and (.goarch | length) > 0) and
  (.real_code.repo | length) > 0 and
  (.real_code.ref | test("^[0-9a-f]{40}$")) and
  (.container.dockerfile | length) > 0 and
  (.container.devcontainer | length) > 0
' "$SPEC" > /dev/null

for term in \
  "make compatibility-gate" \
  "macOS" \
  "Linux" \
  "Container" \
  "minimal"; do
  grep -F "$term" docs/compatibility.md > /dev/null
done

tool_rows=()
while IFS= read -r tool; do
  path="$(command -v "$tool" 2>/dev/null || true)"
  if [ -z "$path" ]; then
    echo "missing required compatibility tool: $tool" >&2
    exit 1
  fi
  row="$OUT/tool-$tool.json"
  jq -n --arg tool "$tool" --arg path "$path" '{tool:$tool,path:$path,found:true}' > "$row"
  tool_rows+=("$row")
done < <(jq -r '.minimal_tools[]' "$SPEC")

build_rows=()
while IFS=$'\t' read -r id goos goarch kind; do
  binary="$OUT/bin/patchline-$id"
  env CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath -o "$binary" ./cmd/patchline
  test -s "$binary"
  size="$(wc -c < "$binary" | tr -d ' ')"
  sha="$(shasum -a 256 "$binary" | awk '{print $1}')"
  runnable=false
  about_path=""
  if [ "$(go env GOOS)" = "$goos" ] && [ "$(go env GOARCH)" = "$goarch" ]; then
    about_path="$OUT/$id-about.txt"
    "$binary" about > "$about_path"
    grep -F "Patchline" "$about_path" > /dev/null
    runnable=true
  fi
  row="$OUT/build-$id.json"
  jq -n \
    --arg id "$id" \
    --arg goos "$goos" \
    --arg goarch "$goarch" \
    --arg kind "$kind" \
    --arg binary "$binary" \
    --arg sha "sha256:$sha" \
    --argjson size "$size" \
    --argjson runnable "$runnable" \
    '{id:$id,kind:$kind,goos:$goos,goarch:$goarch,binary:$binary,size_bytes:$size,sha256:$sha,runnable_on_host:$runnable,verified:true}' > "$row"
  build_rows+=("$row")
done < <(jq -r '.profiles[] | [.id, .goos, .goarch, .kind] | @tsv' "$SPEC")

read -r real_id real_repo real_ref real_subpath < <(jq -r '[.real_code.id, .real_code.repo, .real_code.ref, .real_code.subpath] | @tsv' "$SPEC")
case_out="$OUT/cases/$real_id"
mkdir -p "$case_out"
go run ./cmd/patchline repo analyze \
  --github "$real_repo" \
  --ref "$real_ref" \
  --subpath "$real_subpath" \
  --download-dir "$OUT/cache" \
  --stages inventory,baseline,propose,compare \
  --proposal-kind all \
  --budget files=4,lines=80,tokens=12000,changes=2 \
  --no-llm \
  --out "$case_out/analyze" \
  --json > "$case_out/stdout.json"

jq -e '
  .version == "patchline.repo-analyze/v1" and
  .summary.files_scanned > 0 and
  .summary.ranked_risks > 0 and
  .summary.generated_files > 0 and
  .summary.intervention_loops > 0 and
  (.hash | length) > 0
' "$case_out/analyze/analyze.json" > /dev/null

dockerfile="$(jq -r '.container.dockerfile' "$SPEC")"
devcontainer="$(jq -r '.container.devcontainer' "$SPEC")"
grep -Eq '^FROM golang:' "$dockerfile"
grep -F 'go mod download' "$dockerfile" > /dev/null
grep -F 'CMD ["make", "artifact-smoke"]' "$dockerfile" > /dev/null
jq -e --arg dockerfile "../$dockerfile" '.build.dockerfile == $dockerfile and (.postCreateCommand | length) > 0' "$devcontainer" > /dev/null
while IFS= read -r package; do
  grep -F "$package" "$dockerfile" > /dev/null
done < <(jq -r '.container.required_packages[]' "$SPEC")

container_status="recipe-verified"
container_log="$OUT/container.log"
if command -v docker >/dev/null 2>&1 && docker version --format '{{.Server.Version}}' >/dev/null 2>&1; then
  image="patchline-compat-gate:$(date +%s)"
  docker build -t "$image" . > "$container_log" 2>&1
  docker run --rm "$image" make artifact-smoke >> "$container_log" 2>&1
  container_status="executed"
else
  printf 'docker daemon unavailable; validated Dockerfile and devcontainer recipe instead\n' > "$container_log"
fi

jq -n \
  --slurpfile tools <(jq -s '.' "${tool_rows[@]}") \
  --slurpfile builds <(jq -s '.' "${build_rows[@]}") \
  --slurpfile analyze "$case_out/analyze/analyze.json" \
  --arg container_status "$container_status" \
  --arg container_log "$container_log" \
  '{
    version:"patchline.compatibility-gate-results/v1",
    tools:$tools[0],
    builds:$builds[0],
    real_code:{
      files_scanned:$analyze[0].summary.files_scanned,
      ranked_risks:$analyze[0].summary.ranked_risks,
      generated_files:$analyze[0].summary.generated_files,
      intervention_loops:$analyze[0].summary.intervention_loops,
      hash:$analyze[0].hash
    },
    container:{
      status:$container_status,
      log:$container_log,
      recipe_verified:true
    },
    verified:true
  }' > "$OUT/summary.json"

jq -e '
  .verified == true and
  (.tools | length) >= 5 and
  ([.builds[].goos] | index("darwin")) and
  ([.builds[].goos] | index("linux")) and
  all(.builds[]; .verified == true and .size_bytes > 0 and (.sha256 | startswith("sha256:"))) and
  .real_code.files_scanned > 0 and
  .real_code.ranked_risks > 0 and
  .real_code.generated_files > 0 and
  .container.recipe_verified == true
' "$OUT/summary.json" > /dev/null

echo "compatibility gate passed: $(jq '.builds | length' "$OUT/summary.json") builds, real risks $(jq '.real_code.ranked_risks' "$OUT/summary.json"), container $(jq -r '.container.status' "$OUT/summary.json")"
