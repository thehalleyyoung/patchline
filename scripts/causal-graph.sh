#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/causal-graph-gate.json}"
OUT="${2:-results/generated/causal-graph}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.causal-graph-gate/v1" and (.edges|length) >= 1' "$SPEC" > /dev/null

jq '
  def nodes($edges): [ $edges[] | .from, .to ] | unique;
  # Acyclic iff repeated removal of source nodes (in-degree 0) consumes every node.
  def is_dag($edges):
      (nodes($edges)) as $N
      | reduce range(0; ($N|length)) as $_ ({remaining: $edges, removed: []};
          . as $st
          | ([ $st.remaining[] | .to ] | unique) as $targets
          | ([ (nodes($st.remaining))[] | select([ . == $targets[] ] | any | not) ]) as $sources
          | if ($sources|length) == 0 then .
            else { remaining: [ $st.remaining[] | select([ .from == $sources[] ] | any | not) ],
                   removed: ($st.removed + $sources) } end)
      | (.remaining | length) == 0;
  # Transitive ancestors of $target via reverse reachability over directed edges.
  def ancestors($edges; $target):
      { frontier: [$target], acc: [] }
      | reduce range(0; ($edges|length) + 1) as $_ (.;
          . as $st
          | ([ $edges[] | select([ .to == $st.frontier[] ] | any) | .from ] | unique) as $preds
          | { frontier: $preds, acc: (($st.acc + $preds) | unique) })
      | .acc;
  .failure_node as $f
  | .edges as $E
  | (ancestors($E; $f)) as $anc
  | .correlate_node as $corr
  | {
      version: "patchline.causal-graph/v1",
      failure_node: $f,
      is_dag: is_dag($E),
      root_causes: ($anc | sort),
      correlate_node: $corr,
      correlate_is_cause: (($anc | index($corr)) != null),
      cyclic_is_dag: is_dag(.cyclic_edges)
    }
' "$SPEC" > "$OUT/causal-graph.json"

{
  echo "# Causal-graph root-cause attribution"
  echo
  echo "Root causes of $(jq -r '.failure_node' "$OUT/causal-graph.json"): $(jq -rc '.root_causes' "$OUT/causal-graph.json")"
  echo
  echo "Correlate is a cause: $(jq -r '.correlate_is_cause' "$OUT/causal-graph.json")"
  echo "Cyclic graph is DAG: $(jq -r '.cyclic_is_dag' "$OUT/causal-graph.json")"
} > "$OUT/causal-graph.md"
cp "$OUT/causal-graph.md" "$OUT/README.md"

echo "causal-graph worker: roots=$(jq -rc '.root_causes' "$OUT/causal-graph.json") correlate_cause=$(jq -r '.correlate_is_cause' "$OUT/causal-graph.json") cyclic_dag=$(jq -r '.cyclic_is_dag' "$OUT/causal-graph.json")"
