#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/pre-hook-gate.json}"
OUT="${2:-results/generated/pre-hook-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '
  .version == "patchline.pre-hook-gate/v1" and
  .minimum_public_repos >= 4 and
  ((.required_modes - ["pre-commit", "pre-push"]) | length) == 0 and
  (.slices | length) >= .minimum_public_repos
' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo fetch "$repo" --ref "$ref" --subpath "$subpath" --out "$case_out/fetch" --json > "$case_out/fetch.json"
  scan_root="$(jq -r '.source.scanned_root' "$case_out/fetch.json")"
  test -d "$scan_root"
  work="$case_out/worktree"
  cp -R "$scan_root" "$work"
  git -C "$work" init -q
  git -C "$work" config user.email patchline@example.com
  git -C "$work" config user.name "Patchline Gate"
  git -C "$work" add .
  git -C "$work" commit -q -m "initial real repo slice"
  changed="$(find "$work" -type f \( -name '*.sql' -o -name '*.rb' -o -name '*.go' -o -name '*.py' \) -print0 \
    | xargs -0 grep -El 'UPDATE|DELETE|DROP|ALTER TABLE|remove_column|drop_table|execute' \
    | sed "s#^$work/##" \
    | LC_ALL=C sort \
    | head -n 1)"
  test -n "$changed"
  printf '\n-- patchline hook gate local-only marker\n' >> "$work/$changed"
  git -C "$work" add "$changed"

  go run ./cmd/patchline repo hook pre-commit --root "$work" --out "$case_out/pre-commit" --json > "$case_out/pre-commit.json"
  jq -e '
    .version == "patchline.repo-hook/v1" and
    .mode == "pre-commit" and
    .network == false and
    .summary.network_operations == 0 and
    .summary.changed_files == 1 and
    .summary.scanned_files == 1 and
    .summary.ranked_risks > 0 and
    (.changed_files[0].source == "git-index")
  ' "$case_out/pre-commit.json" > /dev/null

  git -C "$work" commit -q -m "local hook gate change"
  go run ./cmd/patchline repo hook pre-push --root "$work" --base HEAD~1 --out "$case_out/pre-push" --json > "$case_out/pre-push.json"
  jq -e '
    .version == "patchline.repo-hook/v1" and
    .mode == "pre-push" and
    .network == false and
    .summary.network_operations == 0 and
    .summary.changed_files == 1 and
    .summary.scanned_files == 1 and
    .summary.ranked_risks > 0 and
    (.changed_files[0].source == "working-tree")
  ' "$case_out/pre-push.json" > /dev/null

  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --arg changed "$changed" \
    --slurpfile precommit "$case_out/pre-commit.json" \
    --slurpfile prepush "$case_out/pre-push.json" \
    '{
      id:$id,
      repo:$repo,
      subpath:$subpath,
      changed_file:$changed,
      modes:["pre-commit", "pre-push"],
      pre_commit_risks:$precommit[0].summary.ranked_risks,
      pre_push_risks:$prepush[0].summary.ranked_risks,
      network_operations:($precommit[0].summary.network_operations + $prepush[0].summary.network_operations),
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.pre-hook-gate-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_repos:($rows[0] | map(.repo) | unique | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      modes:($rows[0] | map(.modes[]) | unique),
      pre_commit_risks:($rows[0] | map(.pre_commit_risks) | add),
      pre_push_risks:($rows[0] | map(.pre_push_risks) | add),
      network_operations:($rows[0] | map(.network_operations) | add)
    }
  }' > "$OUT/pre-hook-gate.json"

jq -e --slurpfile spec "$SPEC" '
  .summary.public_repos >= $spec[0].minimum_public_repos and
  .summary.verified == (.slices | length) and
  ((($spec[0].required_modes - .summary.modes) | length) == 0) and
  .summary.pre_commit_risks >= (.slices | length) and
  .summary.pre_push_risks >= (.slices | length) and
  .summary.network_operations == 0
' "$OUT/pre-hook-gate.json" > /dev/null

echo "pre-hook gate passed: $(jq '.summary.public_repos' "$OUT/pre-hook-gate.json") public repos, modes=$(jq -r '.summary.modes | join("+")' "$OUT/pre-hook-gate.json"), network_operations=$(jq '.summary.network_operations' "$OUT/pre-hook-gate.json")"
