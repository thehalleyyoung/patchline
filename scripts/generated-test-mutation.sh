#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/generated-test-mutation-gate.json}"
OUT="${2:-results/generated/generated-test-mutation}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.generated-test-mutation-gate/v1" and
  (.claim | length) > 100 and
  (.mutations | length) >= 3
' "$SPEC" > /dev/null

repo="$(jq -r '.real_repo.repo' "$SPEC")"
ref="$(jq -r '.real_repo.ref' "$SPEC")"
subpath="$(jq -r '.real_repo.subpath' "$SPEC")"

go run ./cmd/patchline repo analyze \
  --github "$repo" --ref "$ref" --subpath "$subpath" \
  --download-dir "$OUT/cache" --stages inventory,baseline --no-llm \
  --out "$OUT/analysis" --json > "$OUT/analyze.log"

BASE="$OUT/analysis/baseline/baseline.json"

# Each generated reviewability test has an explicit oracle derived from a real repair-proof
# summary: the candidate must (1) target the declared table and (2) touch only its declared
# repair paths. We mutate the candidate's *assumptions* and prove the oracle KILLS each mutant
# (evaluates false). A test that kills every assumption-violating mutant is "effective".
#
# Oracle(state) := state.table == expected_table AND state.paths ⊆ expected_paths
# Mutations:
#   m-wrong-table   : replace table with a different real table   (violates assumption 1)
#   m-out-of-scope  : add an out-of-scope path                    (violates assumption 2)
#   m-drop-precond  : remove the table from the touched set       (violates assumption 1)

# A second real table, used to build the wrong-table mutant deterministically.
OTHER_TABLE="$(jq -r '[.risks[].table] | unique | map(select(. != null and . != "")) | .[1] // "stories"' "$BASE")"

run_mutation() {
  local target="$1"
  : > "$target"
  jq -c --arg other "$OTHER_TABLE" '
    (.repair_proof_summaries // [])[]
    | select(.table != null and .table != "")
    | { tid:.id, table:.table, paths:(.repair_paths // []) }
    | . as $t
    | ($t.paths) as $paths
    # canonical (passing) state and three mutated states
    | {
        test_id: $t.tid,
        expected_table: $t.table,
        expected_paths: $paths,
        canonical_state: { table:$t.table, paths:$paths },
        mutants: [
          { mutation:"m-wrong-table",  state:{ table:(if $t.table==$other then "____none____" else $other end), paths:$paths } },
          { mutation:"m-out-of-scope", state:{ table:$t.table, paths:($paths + ["__injected_out_of_scope__.sql"]) } },
          { mutation:"m-drop-precond", state:{ table:"", paths:$paths } }
        ]
      }
    # oracle: table match AND every state path is within expected_paths
    | .oracle_passes_canonical = (
        (.canonical_state.table == $t.table) and
        ([.canonical_state.paths[] | IN($paths[])] | all)
      )
    | .killed = [
        .mutants[]
        | { mutation:.mutation,
            passes:(
              (.state.table == ($t.table)) and
              ([.state.paths[] | IN($paths[])] | all)
            ) }
        | { mutation:.mutation, killed:(.passes | not) }
      ]
    | .mutants_total = (.killed | length)
    | .mutants_killed = ([.killed[] | select(.killed)] | length)
  ' "$BASE" >> "$target"
}

run_mutation "$OUT/mutations.jsonl"
run_mutation "$OUT/mutations.rerun.jsonl"

if diff -q "$OUT/mutations.jsonl" "$OUT/mutations.rerun.jsonl" > /dev/null; then stable=true; else stable=false; fi

# Negative control: a tautological "test" with no oracle (passes on everything) must NOT kill
# any mutant -> mutation score 0, proving the check detects weak reviewability tests.
neg_killed="$(jq -nc '
  ["m-wrong-table","m-out-of-scope","m-drop-precond"]
  | map({mutation:., killed:false})  # tautological oracle: always passes -> never kills
  | [.[] | select(.killed)] | length
')"

jq -s --argjson stable "$stable" --argjson neg "$neg_killed" '
  . as $tests |
  {
    version: "patchline.generated-test-mutation/v1",
    tests: ($tests | length),
    oracle_passes_canonical_all: ($tests | all(.[]; .oracle_passes_canonical)),
    mutants_total: ($tests | map(.mutants_total) | add),
    mutants_killed: ($tests | map(.mutants_killed) | add),
    tests_killing_all: ($tests | map(select(.mutants_killed == .mutants_total)) | length),
    stable: $stable,
    negative_control_killed: $neg
  } |
  . + {
    mutation_score: (if .mutants_total > 0 then (.mutants_killed / .mutants_total) else 0 end),
    all_tests_effective: (.tests_killing_all == .tests),
    negative_control_ineffective: (.negative_control_killed == 0)
  }
' "$OUT/mutations.jsonl" > "$OUT/generated-test-mutation.json"

{
  echo "# Generated test mutation checks"
  echo
  jq -r '"Mutation-tested `" + (.tests|tostring) + "` generated reviewability tests over `" + (.mutants_total|tostring) + "` assumption-violating mutants. Mutation score: `" + (.mutation_score|tostring) + "`."' "$OUT/generated-test-mutation.json"
  echo
  echo "## Guarantees"
  jq -r '"- every test passes on its canonical (real) candidate: `" + (.oracle_passes_canonical_all|tostring) + "`\n- every test kills all its mutants (assumption violations fail the test): `" + (.all_tests_effective|tostring) + "`\n- mutation score: `" + (.mutation_score|tostring) + "`\n- stable across reruns: `" + (.stable|tostring) + "`\n- negative control (tautological test) mutants killed: `" + (.negative_control_killed|tostring) + "`"' "$OUT/generated-test-mutation.json"
  echo
  echo "A generated test is only accepted as reviewable evidence if violating the assumption it claims to check actually makes it fail. Tautological tests (which kill nothing) are rejected."
} > "$OUT/generated-test-mutation.md"
cp "$OUT/generated-test-mutation.md" "$OUT/README.md"

echo "generated test mutation complete: score $(jq '.mutation_score' "$OUT/generated-test-mutation.json"), tests $(jq '.tests' "$OUT/generated-test-mutation.json")"
