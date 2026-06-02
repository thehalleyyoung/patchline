#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/symexec-gate.json}"
OUT="${2:-results/generated/symexec}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.symexec-gate/v1" and (.variables|length) >= 1' "$SPEC" > /dev/null

jq '
  .variables as $vars
  # All boolean assignments over the symbolic variables (bounded domain).
  | def assignments:
      reduce $vars[] as $v ([{}];
        [ .[] | . as $a | ($a + {($v): true}), ($a + {($v): false}) ]);
  # First guarded rule whose "when" constraints all hold is the outcome leaf.
  def run($guard; $a):
      ($guard | map(select([.when | to_entries[] | $a[.key] == .value] | all)) | .[0].leaf);
  def explore($guard):
      (assignments) as $space
      | [ $space[] | {assignment: ., leaf: run($guard; .)} ] as $paths
      | {
          reachable_leaves: ([$paths[].leaf] | unique),
          unsafe_reachable: ([$paths[] | select(.leaf == "unsafe")] | length > 0),
          unsafe_witness: ([$paths[] | select(.leaf == "unsafe") | .assignment] | .[0]),
          path_count: ($paths|length)
        };
  {
    version: "patchline.symexec/v1",
    variables: $vars,
    vulnerable: explore(.vulnerable_guard),
    hardened: explore(.hardened_guard)
  }
' "$SPEC" > "$OUT/symexec.json"

{
  echo "# Bounded symbolic execution"
  echo
  echo "Vulnerable guard unsafe reachable: $(jq -r '.vulnerable.unsafe_reachable' "$OUT/symexec.json")"
  echo "Unsafe witness: $(jq -rc '.vulnerable.unsafe_witness' "$OUT/symexec.json")"
  echo
  echo "Hardened guard unsafe reachable: $(jq -r '.hardened.unsafe_reachable' "$OUT/symexec.json")"
} > "$OUT/symexec.md"
cp "$OUT/symexec.md" "$OUT/README.md"

echo "symexec worker: vuln_unsafe=$(jq -r '.vulnerable.unsafe_reachable' "$OUT/symexec.json") hardened_unsafe=$(jq -r '.hardened.unsafe_reachable' "$OUT/symexec.json")"
