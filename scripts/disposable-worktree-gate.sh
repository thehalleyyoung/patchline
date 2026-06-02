#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/disposable-worktree.json}"
OUT="${2:-results/generated/disposable-worktree-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '
  .version == "patchline.disposable-worktree/v1" and
  .minimum_public_slices >= 4 and
  (.proposal_kind | length) > 0 and
  (.budget | test("files=[0-9]+,lines=[0-9]+,tokens=[0-9]+,changes=[0-9]+")) and
  (.claim | contains("disposable Git worktrees"))
' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  budget="$(jq -r '.budget' "$SPEC")"
  proposal_kind="$(jq -r '.proposal_kind' "$SPEC")"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline,propose,compare --proposal-kind "$proposal_kind" --budget "$budget" --no-llm --out "$case_out/analyze" --json > "$case_out/analyze.json"

  source_root="$(jq -r '.scanned_root // .source.scanned_root' "$case_out/analyze/fetch/source.json")"
  worktree="$case_out/worktree"
  mkdir -p "$worktree"
  (cd "$source_root" && tar -cf - .) | (cd "$worktree" && tar -xf -)
  git -C "$worktree" init -q
  git -C "$worktree" -c user.name=Patchline -c user.email=patchline@example.invalid add .
  git -C "$worktree" -c user.name=Patchline -c user.email=patchline@example.invalid commit -qm "baseline"

  while IFS= read -r path; do
    src="$case_out/analyze/proposal/$path"
    dest="$worktree/$path"
    test -s "$src"
    mkdir -p "$(dirname "$dest")"
    cp "$src" "$dest"
  done < <(jq -r '.generated_files[].path' "$case_out/analyze/proposal/proposal.json")
  git -C "$worktree" add -N -- $(jq -r '.generated_files[].path' "$case_out/analyze/proposal/proposal.json")

  git -C "$worktree" diff --binary > "$case_out/worktree.diff"
  git -C "$worktree" diff --name-only | LC_ALL=C sort > "$case_out/worktree-files.txt"
  git -C "$worktree" diff --numstat > "$case_out/worktree-numstat.txt"
  jq -r '.generated_files[].path' "$case_out/analyze/proposal/proposal.json" | LC_ALL=C sort > "$case_out/proposal-files.txt"
  cmp "$case_out/proposal-files.txt" "$case_out/worktree-files.txt"

  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --arg worktree "$worktree" \
    --slurpfile proposal "$case_out/analyze/proposal/proposal.json" \
    --slurpfile compare "$case_out/analyze/compare/compare.json" \
    --rawfile diff "$case_out/worktree.diff" \
    --rawfile numstat "$case_out/worktree-numstat.txt" \
    '{
      id:$id,
      repo:$repo,
      subpath:$subpath,
      disposable_worktree:$worktree,
      generated_files:($proposal[0].generated_files | length),
      diff_files:($numstat | split("\n") | map(select(length > 0)) | length),
      diff_bytes:($diff | length),
      compare_generated_files:$compare[0].summary.generated_files,
      compare_checks_failed:$compare[0].summary.patchline_checks_failed,
      intervention_loops:$compare[0].summary.intervention_loops,
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' examples/real-repo-slices.json)

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.disposable-worktree-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_slices:($rows[0] | length),
      generated_files:($rows[0] | map(.generated_files) | add),
      diff_files:($rows[0] | map(.diff_files) | add),
      failed_checks:($rows[0] | map(.compare_checks_failed) | add)
    }
  }' > "$OUT/disposable-worktrees.json"

jq -e --slurpfile spec "$SPEC" '
  .version == "patchline.disposable-worktree-results/v1" and
  (.slices | length) >= $spec[0].minimum_public_slices and
  .summary.generated_files == .summary.diff_files and
  .summary.failed_checks == 0 and
  all(.slices[]; .verified == true and .generated_files > 0 and .generated_files == .diff_files and .generated_files == .compare_generated_files and .diff_bytes > 0 and .compare_checks_failed == 0 and .intervention_loops > 0)
' "$OUT/disposable-worktrees.json" > /dev/null

echo "disposable worktree gate passed: $(jq '.summary.public_slices' "$OUT/disposable-worktrees.json") public slices produced real generated diffs"
