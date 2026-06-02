#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/taint-tracker-gate.json}"
OUT="${2:-results/generated/taint-tracker}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.taint-tracker-gate/v1" and (.flows|length) >= 1' "$SPEC" > /dev/null

# Forward taint propagation over the inter-procedural flow graph.
# A node is tainted if it is a source, or some unsanitized edge comes from a tainted
# node that is not the inserted sanitizer. Sanitized edges and the sanitizer node block flow.
jq '
  def tainted($sources; $flows; $cut):
    { set: $sources }
    | reduce range(0; ($flows|length) + 1) as $_ (.;
        . as $st
        | ([ $flows[]
             | select(.sanitized == false)
             | select([ .from == $st.set[] ] | any)
             | select(.from != $cut)
             | .to ]) as $new
        | { set: (($st.set + $new) | unique) })
    | .set;
  .sources as $src | .flows as $F | .sinks as $S
  | (tainted($src; $F; "")) as $base
  | (tainted($src; $F; .sanitize_node)) as $cut
  | {
      version: "patchline.taint-tracker/v1",
      sources: $src,
      tainted_sinks: [ $S[] | select([ . == $base[] ] | any) ] | sort,
      tainted_sinks_after_sanitize: [ $S[] | select([ . == $cut[] ] | any) ] | sort,
      tainted_sink: .tainted_sink,
      tainted_sink_reached: ([ .tainted_sink == $base[] ] | any),
      tainted_sink_cut: ([ .tainted_sink == $cut[] ] | any | not),
      clean_sink: .clean_sink,
      clean_sink_tainted: ([ .clean_sink == $base[] ] | any)
    }
' "$SPEC" > "$OUT/taint.json"

{
  echo "# Inter-procedural taint tracking"
  echo
  echo "Tainted migration sinks: $(jq -rc '.tainted_sinks' "$OUT/taint.json")"
  echo "After inserting sanitizer: $(jq -rc '.tainted_sinks_after_sanitize' "$OUT/taint.json")"
} > "$OUT/taint.md"
cp "$OUT/taint.md" "$OUT/README.md"

echo "taint-tracker worker: tainted=$(jq -rc '.tainted_sinks' "$OUT/taint.json") cut=$(jq -r '.tainted_sink_cut' "$OUT/taint.json") clean_ok=$(jq -r '.clean_sink_tainted|not' "$OUT/taint.json")"
