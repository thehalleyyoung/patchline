#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/maintainer-action-simulation-gate.json}"
OUT="${2:-results/generated/maintainer-action-simulation}"
rm -rf "$OUT"
mkdir -p "$OUT/analyses" "$OUT/cache"

jq -e '
  .version == "patchline.maintainer-action-simulation-gate/v1" and
  (.claim | length) > 100 and
  (.required_labels | length) == 5 and
  all(.slices[]; (.repo | contains("/")) and (.ref | test("^[0-9a-f]{40}$")) and (.subpath | length) > 0)
' "$SPEC" > /dev/null

# Deterministic maintainer-action labeler shared across slices.
label_filter='
  . as $b
  | ($b.policy_checks | map({key:.risk_id, value:.}) | from_entries) as $pol
  | ($b.repair_proof_summaries | group_by(.risk_id) | map({key:.[0].risk_id, value:.[0]}) | from_entries) as $rep
  | ($b.proof_hole_minimizations | group_by(.risk_id) | map({key:.[0].risk_id, value:[.[]|(.hole+" "+(.missing_evidence//""))]}) | from_entries) as $holes
  | [ $b.risks[]
      | .id as $rid
      | ($pol[$rid]) as $p
      | ($rep[$rid]) as $r
      | (($holes[$rid] // []) | any(test("row count|runtime|trace|transfer"))) as $runtime
      | (any(.factors[]; .name=="linked-project-evidence")) as $linked
      | (if $runtime then {decision:"needs-runtime-evidence", reason:"confidence blocked on missing runtime evidence (row counts/traces) recorded as proof holes"}
         elif (.severity=="high" and .score>=100 and $linked) then {decision:"accept", reason:"high-severity finding with linked project evidence; act now"}
         elif ($p and $p.status=="fail" and (($p.satisfied|length)>=1)) then {decision:"revise", reason:"partial controls present; revise to add missing guard/rollback/approval/test"}
         elif ($r and $r.status=="conditional") then {decision:"revise", reason:"repair proof is conditional; revise scope/frame obligations"}
         elif (.severity=="low") then {decision:"reject", reason:"low-signal finding; reject/dismiss"}
         else {decision:"defer", reason:"medium-severity finding without urgent or runtime trigger; defer"} end) as $d
      | {risk_id:$rid, path:.path, table:.table, severity:.severity, score:.score, decision:$d.decision, reason:$d.reason}
    ]
'

findings_jsonl="$OUT/findings.jsonl"
: > "$findings_jsonl"
slice_count="$(jq '.slices | length' "$SPEC")"
for ((s=0; s<slice_count; s++)); do
  repo="$(jq -r ".slices[$s].repo" "$SPEC")"
  ref="$(jq -r ".slices[$s].ref" "$SPEC")"
  subpath="$(jq -r ".slices[$s].subpath" "$SPEC")"
  ecosystem="$(jq -r ".slices[$s].ecosystem" "$SPEC")"
  id="$(printf '%s' "${repo//\//-}" | tr -c 'A-Za-z0-9_.-' '-')"
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
  jq -c --arg repo "$repo" --arg ecosystem "$ecosystem" \
    "$label_filter | .[] | . + {repo:\$repo, ecosystem:\$ecosystem}" \
    "$analysis/baseline/baseline.json" >> "$findings_jsonl"
done

jq -s '
  . as $f |
  (["accept","revise","reject","defer","needs-runtime-evidence"]) as $labels |
  {
    version: "patchline.maintainer-action-simulation/v1",
    label_distribution: ($labels | map(. as $l | {label:$l, count:([$f[] | select(.decision==$l)] | length)})),
    by_repository: ($f | group_by(.repo) | map(. as $g | {
      repo: $g[0].repo,
      ecosystem: $g[0].ecosystem,
      findings: ($g | length),
      labels: ($labels | map(. as $l | {(($l)):([$g[] | select(.decision==$l)] | length)}) | add)
    })),
    summary: {
      findings: ($f | length),
      repositories: ($f | map(.repo) | unique | length),
      labels_present: ([$labels[] | select(. as $l | any($f[]; .decision==$l))] | length),
      all_labels_present: (all($labels[]; . as $l | any($f[]; .decision==$l))),
      verified: (($f | length) > 0 and all($labels[]; . as $l | any($f[]; .decision==$l)))
    }
  }
' "$findings_jsonl" > "$OUT/maintainer-action-simulation.json"

{
  echo "# Maintainer-action simulation"
  echo
  echo "Patchline assigns each ranked finding a simulated maintainer decision (accept, revise, reject, defer, needs-runtime-evidence) from deterministic signals: severity, linked project evidence, policy controls, repair proof status, and runtime proof holes."
  echo
  echo "## Label distribution"
  echo
  echo "| Decision | Findings |"
  echo "| --- | ---: |"
  jq -r '.label_distribution[] | "| " + .label + " | " + (.count|tostring) + " |"' "$OUT/maintainer-action-simulation.json"
  echo
  echo "## By repository"
  echo
  echo "| Repo | Ecosystem | Findings | accept | revise | reject | defer | needs-runtime-evidence |"
  echo "| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |"
  jq -r '.by_repository[] | "| `" + .repo + "` | " + .ecosystem + " | " + (.findings|tostring) + " | " + (.labels.accept|tostring) + " | " + (.labels.revise|tostring) + " | " + (.labels.reject|tostring) + " | " + (.labels.defer|tostring) + " | " + (.labels["needs-runtime-evidence"]|tostring) + " |"' "$OUT/maintainer-action-simulation.json"
} > "$OUT/maintainer-action-simulation.md"

cp "$OUT/maintainer-action-simulation.md" "$OUT/README.md"
echo "maintainer-action simulation complete: findings $(jq '.summary.findings' "$OUT/maintainer-action-simulation.json"), labels present $(jq '.summary.labels_present' "$OUT/maintainer-action-simulation.json")/5"
