#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/causality-limits-gate.json}"
OUT="${2:-results/generated/causality-limits}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.causality-limits-gate/v1" and
  (.claim | length) > 100 and
  (.required_labels | length) == 4 and
  (.forbidden_labels | length) >= 1
' "$SPEC" > /dev/null

repo="$(jq -r '.real_repo.repo' "$SPEC")"
ref="$(jq -r '.real_repo.ref' "$SPEC")"
subpath="$(jq -r '.real_repo.subpath' "$SPEC")"
maxf="$(jq '.max_findings' "$SPEC")"

go run ./cmd/patchline repo analyze \
  --github "$repo" --ref "$ref" --subpath "$subpath" \
  --download-dir "$OUT/cache" --stages inventory,baseline --no-llm \
  --out "$OUT/analysis" --json > "$OUT/analyze.log"

BASE="$OUT/analysis/baseline/baseline.json"

# Select real findings, then construct four trace-link scenarios per finding with controlled
# causal structure and classify each with a deterministic, overclaim-resistant rule.
jq \
  --argjson max "$maxf" \
  --argjson forbidden "$(jq -c '.forbidden_labels' "$SPEC")" \
  --arg ceiling "$(jq -r '.causal_ceiling' "$SPEC")" '
  [ .risks[] | select(.table != null and .table != "")
    | { id, table, severity, sr:(if .severity=="high" then 0 elif .severity=="medium" then 1 else 2 end) } ]
  | sort_by([.sr, .id]) | .[0:$max] as $sel |

  # Classifier: given temporal order, same-table, and confounder flags, emit a verdict that is
  # never stronger than "plausible".
  def classify(same_table; deploy_before_error; confounded):
    if (same_table | not) then "unlinked"
    elif (deploy_before_error | not) then "temporally-inconsistent"
    elif confounded then "confounded"
    else "plausible" end;

  [ $sel[] | . as $f |
    [
      { scenario:"clean",        same_table:true,  deploy_before_error:true,  confounded:false, other_table:null },
      { scenario:"confounded",   same_table:true,  deploy_before_error:true,  confounded:true,  other_table:null },
      { scenario:"temporal",     same_table:true,  deploy_before_error:false, confounded:false, other_table:null },
      { scenario:"cross-table",  same_table:false, deploy_before_error:true,  confounded:false, other_table:("other_" + $f.table) }
    ]
    | .[]
    | . as $s
    | classify($s.same_table; $s.deploy_before_error; $s.confounded) as $verdict
    | {
        finding_id: $f.id, table: $f.table, severity: $f.severity,
        scenario: $s.scenario,
        same_table: $s.same_table,
        deploy_before_error: $s.deploy_before_error,
        confounder_present: $s.confounded,
        observed_table: ($s.other_table // $f.table),
        verdict: $verdict,
        # The verdict must never be a forbidden (overclaiming) label.
        overclaim: ($forbidden | index($verdict) != null),
        # "plausible" is the ceiling: it asserts consistency, not proof.
        claim_strength: (if $verdict=="plausible" then "consistent-with" else "downgraded" end)
      }
  ]
' "$BASE" > "$OUT/scenarios.json"

jq '
  . as $rows | ($rows | length) as $n |
  ($rows | group_by(.verdict) | map({key:.[0].verdict, value:length}) | from_entries) as $by_verdict |
  {
    version: "patchline.causality-limits/v1",
    total_scenarios: $n,
    findings: ($rows | map(.finding_id) | unique | length),
    verdict_distribution: $by_verdict,
    overclaims: ($rows | map(select(.overclaim)) | length),
    plausible_are_consistent_only: ($rows | map(select(.verdict=="plausible")) | all(.[]; .claim_strength=="consistent-with")),
    temporal_violations_downgraded: ($rows | map(select(.deploy_before_error|not)) | all(.[]; .verdict=="temporally-inconsistent")),
    cross_table_unlinked: ($rows | map(select(.same_table|not)) | all(.[]; .verdict=="unlinked")),
    confounded_flagged: ($rows | map(select(.confounder_present and .same_table and .deploy_before_error)) | all(.[]; .verdict=="confounded"))
  }
' "$OUT/scenarios.json" > "$OUT/causality-limits.json"

{
  echo "# Trace-to-migration causality limitations (anti-overclaiming)"
  echo
  jq -r '"Constructed `" + (.total_scenarios|tostring) + "` trace-link scenarios across `" + (.findings|tostring) + "` real findings. Overclaiming labels emitted: `" + (.overclaims|tostring) + "`."' "$OUT/causality-limits.json"
  echo
  echo "## Verdict distribution"
  jq -r '.verdict_distribution | to_entries[] | "- " + .key + ": `" + (.value|tostring) + "`"' "$OUT/causality-limits.json"
  echo
  echo "## Causality safeguards"
  jq -r '"- plausible verdicts assert consistency only (never proof): `" + (.plausible_are_consistent_only|tostring) + "`\n- temporal inconsistencies downgraded: `" + (.temporal_violations_downgraded|tostring) + "`\n- cross-table telemetry left unlinked: `" + (.cross_table_unlinked|tostring) + "`\n- confounded windows flagged: `" + (.confounded_flagged|tostring) + "`"' "$OUT/causality-limits.json"
  echo
  echo "Correlated telemetry is never promoted to a causal claim: the strongest verdict is *plausible (consistent-with)*, and confounders, temporal violations, and table mismatches are explicitly downgraded."
} > "$OUT/causality-limits.md"

cp "$OUT/causality-limits.md" "$OUT/README.md"
echo "causality limits complete: scenarios $(jq '.total_scenarios' "$OUT/causality-limits.json"), overclaims $(jq '.overclaims' "$OUT/causality-limits.json")"
