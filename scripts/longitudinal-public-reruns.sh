#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/longitudinal-public-reruns-gate.json}"
OUT="${2:-results/generated/longitudinal-public-reruns}"
rm -rf "$OUT"
mkdir -p "$OUT/analyses" "$OUT/cache"

jq -e '
  . as $root |
  .version == "patchline.longitudinal-public-reruns-gate/v1" and
  (.claim | length) > 100 and
  (.slices | length) >= .minimum_repositories and
  all(.slices[]; (.repo | contains("/")) and (.subpath | length) > 0 and (.commits | length) >= $root.minimum_commits_per_repository and all(.commits[]; (.ref | test("^[0-9a-f]{40}$")) and (.date | test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T"))))
' "$SPEC" > /dev/null

run_rows=()
run_jsonl="$OUT/runs.jsonl"
: > "$run_jsonl"

slice_count="$(jq '.slices | length' "$SPEC")"
for ((s=0; s<slice_count; s++)); do
  repo="$(jq -r ".slices[$s].repo" "$SPEC")"
  subpath="$(jq -r ".slices[$s].subpath" "$SPEC")"
  ecosystem="$(jq -r ".slices[$s].ecosystem" "$SPEC")"
  framework="$(jq -r ".slices[$s].framework" "$SPEC")"
  commit_count="$(jq ".slices[$s].commits | length" "$SPEC")"
  for ((c=0; c<commit_count; c++)); do
    ref="$(jq -r ".slices[$s].commits[$c].ref" "$SPEC")"
    date="$(jq -r ".slices[$s].commits[$c].date" "$SPEC")"
    label="$(jq -r ".slices[$s].commits[$c].label" "$SPEC")"
    id="$(printf '%s-%s' "${repo//\//-}" "$label" | tr -c 'A-Za-z0-9_.-' '-')"
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
    row="$OUT/run-$id.json"
    jq -n \
      --arg id "$id" \
      --arg repo "$repo" \
      --arg subpath "$subpath" \
      --arg ecosystem "$ecosystem" \
      --arg framework "$framework" \
      --arg ref "$ref" \
      --arg date "$date" \
      --arg label "$label" \
      --arg analysis "$analysis" \
      --slurpfile analyze "$analysis/analyze.json" \
      '{
        id:$id,
        repo:$repo,
        subpath:$subpath,
        ecosystem:$ecosystem,
        framework:$framework,
        ref:$ref,
        date:$date,
        label:$label,
        analysis:$analysis,
        files_scanned:$analyze[0].summary.files_scanned,
        ranked_risks:$analyze[0].summary.ranked_risks,
        provenance_slices:$analyze[0].summary.provenance_slices,
        generated_files:($analyze[0].summary.generated_files // 0),
        hash:$analyze[0].hash,
        verified:(($analyze[0].summary.files_scanned > 0) and ($analyze[0].summary.ranked_risks >= 0) and (($analyze[0].hash | length) > 0))
      }' > "$row"
    jq -c . "$row" >> "$run_jsonl"
    run_rows+=("$row")
  done
done

jq -s '
  group_by(.repo) |
  map(sort_by(.date) |
    {
      repo:.[0].repo,
      subpath:.[0].subpath,
      ecosystem:.[0].ecosystem,
      framework:.[0].framework,
      commits:length,
      first_ref:.[0].ref,
      last_ref:.[-1].ref,
      first_date:.[0].date,
      last_date:.[-1].date,
      first_ranked_risks:.[0].ranked_risks,
      last_ranked_risks:.[-1].ranked_risks,
      risk_delta:(.[-1].ranked_risks - .[0].ranked_risks),
      files_delta:(.[-1].files_scanned - .[0].files_scanned),
      provenance_delta:(.[-1].provenance_slices - .[0].provenance_slices),
      hashes:map(.hash),
      verified:all(.[]; .verified == true)
    }
  )' "${run_rows[@]}" > "$OUT/repository-trends.json"

jq -n \
  --slurpfile runs <(jq -s '.' "${run_rows[@]}") \
  --slurpfile trends "$OUT/repository-trends.json" \
  '{
    version:"patchline.longitudinal-public-reruns/v1",
    runs:$runs[0],
    repository_trends:$trends[0],
    summary:{
      repositories:($trends[0] | length),
      total_runs:($runs[0] | length),
      commits_per_repository_min:($trends[0] | map(.commits) | min),
      files_scanned:([$runs[0][].files_scanned] | add),
      ranked_risks:([$runs[0][].ranked_risks] | add),
      provenance_slices:([$runs[0][].provenance_slices] | add),
      repositories_with_risk_delta:($trends[0] | map(select(.risk_delta != 0)) | length),
      verified:(all($runs[0][]; .verified == true) and all($trends[0][]; .verified == true))
    }
  }' > "$OUT/longitudinal-reruns.json"

{
  echo "# Longitudinal public-corpus reruns"
  echo
  echo "Patchline reran the same public repository slices over multiple pinned historical commits to measure how files, ranked risks, and evidence counts change over time."
  echo
  echo "## Summary"
  jq -r '.summary | "- repositories: `" + (.repositories|tostring) + "`\n- total runs: `" + (.total_runs|tostring) + "`\n- ranked risks: `" + (.ranked_risks|tostring) + "`\n- provenance slices: `" + (.provenance_slices|tostring) + "`\n- repositories with risk delta: `" + (.repositories_with_risk_delta|tostring) + "`"' "$OUT/longitudinal-reruns.json"
  echo
  echo "## Repository trends"
  echo
  echo "| Repo | Commits | First ref | Last ref | Risk delta | Files delta | Provenance delta |"
  echo "| --- | ---: | --- | --- | ---: | ---: | ---: |"
  jq -r '.repository_trends[] | "| `" + .repo + "` | " + (.commits|tostring) + " | `" + (.first_ref[0:12]) + "` | `" + (.last_ref[0:12]) + "` | " + (.risk_delta|tostring) + " | " + (.files_delta|tostring) + " | " + (.provenance_delta|tostring) + " |"' "$OUT/longitudinal-reruns.json"
} > "$OUT/longitudinal-reruns.md"

cp "$OUT/longitudinal-reruns.md" "$OUT/README.md"
echo "longitudinal public reruns complete: repos $(jq '.summary.repositories' "$OUT/longitudinal-reruns.json"), runs $(jq '.summary.total_runs' "$OUT/longitudinal-reruns.json"), risks $(jq '.summary.ranked_risks' "$OUT/longitudinal-reruns.json")"
