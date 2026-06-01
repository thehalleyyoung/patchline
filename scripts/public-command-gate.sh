#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATES="${1:-examples/public-command-gates.json}"
OUT="${2:-results/generated/public-command-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.public-command-gates/v1" and
  (.gates | length) >= 4 and
  all(.gates[];
    (.id | length) > 0 and
    (.real_repo | length) > 0 and
    (.subpath | length) > 0 and
    (.max_shell_commands > 0 and .max_shell_commands <= 3) and
    (.command_claim | length) > 30
  )
' "$GATES" > /dev/null

jq -n --slurpfile gates "$GATES" --slurpfile slices examples/real-repo-slices.json '
  ($slices[0].slices | map(.repo + ":" + .subpath)) as $known |
  $gates[0].gates
  | all(.[]; (.real_repo + ":" + .subpath) as $key | ($known | index($key)))
' | jq -e '. == true' > /dev/null

rows=()
while IFS=$'\t' read -r id repo subpath max_shell_commands command_claim; do
  case_out="$OUT/$id"
  mkdir -p "$case_out"
  ref="$(jq -r --arg repo "$repo" --arg subpath "$subpath" '.slices[] | select(.repo == $repo and .subpath == $subpath) | .ref' examples/real-repo-slices.json)"
  test -n "$ref"

  {
    printf 'go run ./cmd/patchline repo fetch %q --ref %q --subpath %q --out %q --json > %q\n' "$repo" "$ref" "$subpath" "$case_out/fetch" "$case_out/fetch.json"
    printf 'SCAN_ROOT="$(jq -r .source.scanned_root %q)"\n' "$case_out/fetch.json"
    printf 'go run ./cmd/patchline repo analyze "$SCAN_ROOT" --stages inventory,baseline,propose,compare --proposal-kind all --budget files=4,lines=100,tokens=10000,changes=2 --no-llm --out %q --json > %q\n' "$case_out/analyze" "$case_out/analyze.json"
  } > "$case_out/commands.sh"

  go run ./cmd/patchline repo fetch "$repo" --ref "$ref" --subpath "$subpath" --out "$case_out/fetch" --json > "$case_out/fetch.json"
  scan_root="$(jq -r '.source.scanned_root' "$case_out/fetch.json")"
  test -d "$scan_root"
  go run ./cmd/patchline repo analyze "$scan_root" --stages inventory,baseline,propose,compare --proposal-kind all --budget files=4,lines=100,tokens=10000,changes=2 --no-llm --out "$case_out/analyze" --json > "$case_out/analyze.json"

  test -s "$case_out/analyze/commands.md"
  grep -Fq "repo analyze" "$case_out/analyze/commands.md"
  jq -e '.summary.ranked_risks > 0 and .summary.generated_files > 0 and .summary.intervention_loops > 0 and .summary.deterministic_only == true' "$case_out/analyze.json" > /dev/null

  command_count="$(grep -Ec '^(go run|SCAN_ROOT=)' "$case_out/commands.sh")"
  test "$command_count" -le "$max_shell_commands"
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --arg scan_root "$scan_root" \
    --arg command_claim "$command_claim" \
    --argjson command_count "$command_count" \
    --argjson max_shell_commands "$max_shell_commands" \
    --slurpfile analyze "$case_out/analyze.json" \
    '{id: $id, repo: $repo, subpath: $subpath, scan_root: $scan_root, command_count: $command_count, max_shell_commands: $max_shell_commands, command_claim: $command_claim, ranked_risks: $analyze[0].summary.ranked_risks, generated_files: $analyze[0].summary.generated_files, intervention_loops: $analyze[0].summary.intervention_loops, verified: true}' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.gates[] | [.id, .real_repo, .subpath, .max_shell_commands, .command_claim] | @tsv' "$GATES")

jq -s '{version:"patchline.public-command-gate-results/v1", gates: .}' "${rows[@]}" > "$OUT/summary.json"
jq -e '(.gates | length) >= 4 and all(.gates[]; .verified == true and .command_count <= .max_shell_commands and .ranked_risks > 0 and .intervention_loops > 0)' "$OUT/summary.json" > /dev/null
echo "public-command gate passed: $(jq '.gates | length' "$OUT/summary.json") public repo slices ran from downloaded paths in short shell sequences"
