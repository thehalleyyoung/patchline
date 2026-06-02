#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/intervention-scorecard-gate.json}"
OUT="${2:-results/generated/intervention-scorecard}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.intervention-scorecard-gate/v1" and
  (.claim | length) > 100 and
  (.dimensions | length) == 4
' "$SPEC" > /dev/null

repo="$(jq -r '.real_repo.repo' "$SPEC")"
ref="$(jq -r '.real_repo.ref' "$SPEC")"
subpath="$(jq -r '.real_repo.subpath' "$SPEC")"

go run ./cmd/patchline repo analyze \
  --github "$repo" --ref "$ref" --subpath "$subpath" \
  --download-dir "$OUT/cache" --stages inventory,baseline --no-llm \
  --out "$OUT/analysis" --json > "$OUT/analyze.log"

BASE="$OUT/analysis/baseline/baseline.json"

# Build a reviewer scorecard per intervention (one per repair-proof summary). The four axes are
# kept SEPARATE and never collapsed into a single number:
#   usefulness    -- how much risk it addresses (normalized risk score + evidence breadth)
#   safety        -- absence of deterministic rejection signals on the linked risk
#   completeness  -- whether scope+frame obligations are met and rollback paths exist
#   uncertainty   -- proportion of open proof holes (higher = less certain)
score() {
  jq -c '
    ([.risks[]] | INDEX(.id)) as $byrisk
    | ([.risks[].score] | max) as $maxscore
    | (.repair_proof_summaries // [])[]
    | . as $p
    | ($byrisk[$p.risk_id] // {score:0, factors:[]}) as $risk
    | ([$risk.factors[]?.name]) as $factors
    | ($p.proof_holes // []) as $holes
    | ($p.repair_paths // []) as $paths
    | ($p.evidence // []) as $evidence
    # usefulness: normalized risk score blended with evidence breadth (capped)
    | (( ($risk.score / (if $maxscore>0 then $maxscore else 1 end)) * 0.7 )
        + (([($evidence|length),10]|min) / 10 * 0.3)) as $useful
    # safety: 1 minus the share of rejection-signal families present (4 families)
    | ([
         ($factors | any(. == "high-risk-sql" or . == "destructive-effect" or . == "destructive-code-path")),
         ($factors | any(. == "broad-write" or . == "schema-write-breadth" or . == "write-breadth-unknown")),
         ($factors | any(. == "weak-rollback-signal")),
         ($factors | any(. == "missing-idempotency" or . == "missing-transaction-boundary"))
       ] | map(select(.)) | length) as $rejfamilies
    | (1 - ($rejfamilies / 4)) as $safety
    # completeness: scope+frame both pass AND at least one rollback path
    | (((if $p.scope_status == "pass" then 0.4 else 0 end)
        + (if $p.frame_status == "pass" then 0.4 else 0 end)
        + (if ($paths|length) > 0 then 0.2 else 0 end))) as $complete
    # uncertainty: share of open proof holes (capped at 4)
    | (([($holes|length),4]|min) / 4) as $uncert
    | {
        intervention_id: $p.id,
        risk_id: $p.risk_id,
        table: $p.table,
        scorecard: {
          usefulness: (($useful*100|round)/100),
          safety: (($safety*100|round)/100),
          completeness: (($complete*100|round)/100),
          uncertainty: (($uncert*100|round)/100)
        },
        open_proof_holes: ($holes|length)
      }
    # honesty invariant: open proof holes must be reflected in the uncertainty axis, and a card
    # must never claim full certainty (completeness 1 AND uncertainty 0) while holes remain.
    | . + { honest: ( (.open_proof_holes == 0) or (.scorecard.uncertainty > 0) ) }
  ' "$BASE"
}

score > "$OUT/scorecards.jsonl"
score > "$OUT/scorecards.rerun.jsonl"
if diff -q "$OUT/scorecards.jsonl" "$OUT/scorecards.rerun.jsonl" > /dev/null; then stable=true; else stable=false; fi

jq -s --argjson stable "$stable" '
  . as $cards |
  ($cards | length) as $n |
  {
    version: "patchline.intervention-scorecard/v1",
    interventions: $n,
    dimensions: ["usefulness","safety","completeness","uncertainty"],
    all_in_unit_range: ($cards | all(.[]; (.scorecard | to_entries | all(.value >= 0 and .value <= 1)))),
    all_four_present: ($cards | all(.[]; (.scorecard | keys | sort) == ["completeness","safety","uncertainty","usefulness"])),
    all_honest: ($cards | all(.[]; .honest)),
    # axes are genuinely separate: across the corpus at least two axes must differ for some card
    axes_separable: ($cards | any(.[]; (.scorecard.safety != .scorecard.completeness) or (.scorecard.usefulness != .scorecard.uncertainty))),
    cards_with_open_holes: ($cards | map(select(.open_proof_holes > 0)) | length),
    overclaimed_completeness: ($cards | map(select(.open_proof_holes > 0 and .scorecard.completeness >= 1 and .scorecard.uncertainty == 0)) | length),
    means: {
      usefulness: (($cards | map(.scorecard.usefulness) | add / $n * 1000 | round)/1000),
      safety: (($cards | map(.scorecard.safety) | add / $n * 1000 | round)/1000),
      completeness: (($cards | map(.scorecard.completeness) | add / $n * 1000 | round)/1000),
      uncertainty: (($cards | map(.scorecard.uncertainty) | add / $n * 1000 | round)/1000)
    },
    stable: $stable
  }
' "$OUT/scorecards.jsonl" > "$OUT/intervention-scorecard.json"

{
  echo "# Reviewer intervention scorecards"
  echo
  jq -r '"Scored `" + (.interventions|tostring) + "` real interventions on four separate axes (usefulness, safety, completeness, uncertainty)."' "$OUT/intervention-scorecard.json"
  echo
  echo "## Corpus means"
  jq -r '.means | "- usefulness: `" + (.usefulness|tostring) + "`\n- safety: `" + (.safety|tostring) + "`\n- completeness: `" + (.completeness|tostring) + "`\n- uncertainty: `" + (.uncertainty|tostring) + "`"' "$OUT/intervention-scorecard.json"
  echo
  echo "## Guarantees"
  jq -r '"- all four axes present per card: `" + (.all_four_present|tostring) + "`\n- every score in [0,1]: `" + (.all_in_unit_range|tostring) + "`\n- axes are separable (not one collapsed number): `" + (.axes_separable|tostring) + "`\n- no card claims full certainty while proof holes remain: `" + ((.overclaimed_completeness == 0)|tostring) + "`\n- stable across reruns: `" + (.stable|tostring) + "`"' "$OUT/intervention-scorecard.json"
  echo
  echo "Each scorecard keeps usefulness, safety, completeness, and uncertainty distinct so a reviewer can see, for example, a useful-but-uncertain intervention rather than a single misleading overall grade."
} > "$OUT/intervention-scorecard.md"
cp "$OUT/intervention-scorecard.md" "$OUT/README.md"

echo "intervention scorecard complete: $(jq '.interventions' "$OUT/intervention-scorecard.json") cards, overclaimed $(jq '.overclaimed_completeness' "$OUT/intervention-scorecard.json")"
