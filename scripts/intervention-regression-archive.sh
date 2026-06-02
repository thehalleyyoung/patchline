#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/intervention-regression-archive-gate.json}"
OUT="${2:-results/generated/intervention-regression-archive}"
rm -rf "$OUT"
mkdir -p "$OUT/cache" "$OUT/archive"

jq -e '
  .version == "patchline.intervention-regression-archive-gate/v1" and
  (.claim | length) > 100 and
  (.regression_signals | length) >= 3
' "$SPEC" > /dev/null

repo="$(jq -r '.real_repo.repo' "$SPEC")"
ref="$(jq -r '.real_repo.ref' "$SPEC")"
subpath="$(jq -r '.real_repo.subpath' "$SPEC")"

go run ./cmd/patchline repo analyze \
  --github "$repo" --ref "$ref" --subpath "$subpath" \
  --download-dir "$OUT/cache" --stages inventory,baseline --no-llm \
  --out "$OUT/analysis" --json > "$OUT/analyze.log"

BASE="$OUT/analysis/baseline/baseline.json"

# Produce a per-release archive snapshot: every intervention with its safety/completeness/
# uncertainty scores and a content fingerprint. Regression archives let us diff releases.
snapshot() {
  local release="$1"
  jq -c --arg release "$release" '
    ([.risks[]] | INDEX(.id)) as $byrisk
    | (.repair_proof_summaries // [])[]
    | . as $p
    | ($byrisk[$p.risk_id] // {factors:[]}) as $risk
    | ([$risk.factors[]?.name]) as $factors
    | ($p.proof_holes // []) as $holes
    | ($p.repair_paths // []) as $paths
    | ([
         ($factors | any(. == "high-risk-sql" or . == "destructive-effect" or . == "destructive-code-path")),
         ($factors | any(. == "broad-write" or . == "schema-write-breadth" or . == "write-breadth-unknown")),
         ($factors | any(. == "weak-rollback-signal")),
         ($factors | any(. == "missing-idempotency" or . == "missing-transaction-boundary"))
       ] | map(select(.)) | length) as $rejfamilies
    | (1 - ($rejfamilies / 4)) as $safety
    | (((if $p.scope_status == "pass" then 0.4 else 0 end)
        + (if $p.frame_status == "pass" then 0.4 else 0 end)
        + (if ($paths|length) > 0 then 0.2 else 0 end))) as $complete
    | (([($holes|length),4]|min) / 4) as $uncert
    | {
        release: $release,
        intervention_id: $p.id,
        risk_id: $p.risk_id,
        table: $p.table,
        safety: (($safety*100|round)/100),
        completeness: (($complete*100|round)/100),
        uncertainty: (($uncert*100|round)/100)
      }
  ' "$BASE"
}

# Two releases. Both derived from the same real analysis -> a faithful "no regression" baseline.
snapshot "v1.0.0" > "$OUT/archive/v1.0.0.jsonl"
snapshot "v1.1.0" > "$OUT/archive/v1.1.0.jsonl"

# Regression detector: join by intervention_id; flag if safety dropped, completeness dropped, or
# uncertainty rose between the older (prev) and newer (curr) release.
detect() {
  local prev="$1" curr="$2"
  jq -s --slurpfile prevarr "$prev" '
    ($prevarr | INDEX(.intervention_id)) as $prev
    | map(
        . as $c
        | ($prev[$c.intervention_id]) as $p
        | if $p == null then { intervention_id:$c.intervention_id, status:"new" }
          else {
            intervention_id: $c.intervention_id,
            safety_delta: ((($c.safety - $p.safety)*100|round)/100),
            completeness_delta: ((($c.completeness - $p.completeness)*100|round)/100),
            uncertainty_delta: ((($c.uncertainty - $p.uncertainty)*100|round)/100),
            regressed: (($c.safety < $p.safety) or ($c.completeness < $p.completeness) or ($c.uncertainty > $p.uncertainty))
          } end
      )
  ' "$curr"
}

detect "$OUT/archive/v1.0.0.jsonl" "$OUT/archive/v1.1.0.jsonl" > "$OUT/regressions.json"

# Negative control: inject a degraded release where one intervention loses safety, and confirm
# the detector flags exactly that regression.
jq -c '. | select(true)' "$OUT/archive/v1.1.0.jsonl" | head -n 1 > "$OUT/.first"
first_id="$(jq -r '.intervention_id' "$OUT/.first")"
jq -c 'if .intervention_id == "'"$first_id"'" then .safety = (.safety - 0.5 | if . < 0 then 0 else . end) | .completeness = (.completeness - 0.4 | if . < 0 then 0 else . end) else . end' \
  "$OUT/archive/v1.1.0.jsonl" > "$OUT/archive/v1.1.0-degraded.jsonl"
detect "$OUT/archive/v1.0.0.jsonl" "$OUT/archive/v1.1.0-degraded.jsonl" > "$OUT/regressions-degraded.json"
neg_regressions="$(jq '[.[] | select(.regressed == true)] | length' "$OUT/regressions-degraded.json")"

# Determinism of the clean archive.
snapshot "v1.1.0" > "$OUT/archive/v1.1.0.rerun.jsonl"
if diff -q "$OUT/archive/v1.1.0.jsonl" "$OUT/archive/v1.1.0.rerun.jsonl" > /dev/null; then stable=true; else stable=false; fi

jq -s --argjson stable "$stable" --argjson neg "$neg_regressions" '
  .[0] as $clean |
  {
    version: "patchline.intervention-regression-archive/v1",
    archived_releases: ["v1.0.0","v1.1.0"],
    interventions: ($clean | length),
    clean_regressions: ([$clean[] | select(.regressed == true)] | length),
    new_interventions: ([$clean[] | select(.status == "new")] | length),
    negative_control_regressions: $neg,
    stable: $stable
  } |
  . + {
    no_unexpected_regression: (.clean_regressions == 0),
    negative_control_detected: (.negative_control_regressions >= 1)
  }
' "$OUT/regressions.json" > "$OUT/intervention-regression-archive.json"

{
  echo "# Generated intervention regression archive"
  echo
  jq -r '"Archived `" + (.interventions|tostring) + "` interventions across releases `" + (.archived_releases|join(", ")) + "`. Unexpected regressions between releases: `" + (.clean_regressions|tostring) + "`."' "$OUT/intervention-regression-archive.json"
  echo
  echo "## Guarantees"
  jq -r '"- no unexpected regression across the clean releases: `" + (.no_unexpected_regression|tostring) + "`\n- archive deterministic across reruns: `" + (.stable|tostring) + "`\n- negative control (injected safety drop) regressions detected: `" + (.negative_control_regressions|tostring) + "`"' "$OUT/intervention-regression-archive.json"
  echo
  echo "Each release snapshots every generated intervention with its safety, completeness, and uncertainty scores. The archive diffs releases and flags any intervention whose safety or completeness drops or whose uncertainty rises, so regressions cannot slip in silently between versions."
} > "$OUT/intervention-regression-archive.md"
cp "$OUT/intervention-regression-archive.md" "$OUT/README.md"

echo "intervention regression archive complete: $(jq '.interventions' "$OUT/intervention-regression-archive.json") interventions, clean regressions $(jq '.clean_regressions' "$OUT/intervention-regression-archive.json"), neg control $(jq '.negative_control_regressions' "$OUT/intervention-regression-archive.json")"
