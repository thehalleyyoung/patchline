#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/severity-calibration-gate.json}"
OUT="${2:-results/generated/severity-calibration}"
rm -rf "$OUT"
mkdir -p "$OUT/analyses" "$OUT/cache"

jq -e '
  .version == "patchline.severity-calibration-gate/v1" and
  (.claim | length) > 100 and
  (.required_severities | length) == 3 and
  all(.slices[]; (.repo | contains("/")) and (.ref | test("^[0-9a-f]{40}$")) and (.subpath | length) > 0)
' "$SPEC" > /dev/null

# For each risk decide whether independent danger evidence corroborates it:
# incident/cause clusters, rollback/fix repair clusters, and recurring-hazard signals,
# matched by the table the risk touches.
corroborate_filter='
  ([.cause_clusters[]?, .repair_clusters[]?, .recurrences[]?]
    | map([(.identifiers[]? | select(.kind=="table") | .value), (.table? // empty)])
    | flatten | map(select(. != null and . != "")) | unique) as $danger |
  ([.cause_clusters[]?] | length) as $incident_signals |
  ([.repair_clusters[]?] | length) as $rollback_signals |
  ([.recurrences[]?] | length) as $recurrence_signals |
  [ .risks[]
    | .table as $t
    | {
        risk_id: .id,
        path: .path,
        table: ($t // ""),
        severity: .severity,
        score: .score,
        corroborated: (($t != null) and (($danger | index($t)) != null))
      }
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
    "$corroborate_filter | .[] | . + {repo:\$repo, ecosystem:\$ecosystem}" \
    "$analysis/baseline/baseline.json" >> "$findings_jsonl"
done

jq -s '
  . as $f |
  def rate($rows): if ($rows|length) > 0 then (([$rows[]|select(.corroborated)]|length) / ($rows|length)) else 0 end;
  ([$f[]|select(.severity=="high" or .severity=="medium")]) as $elevated |
  ([$f[]|select(.severity=="low")]) as $low |
  (rate($elevated)) as $elevated_rate |
  (rate($low)) as $low_rate |
  {
    version: "patchline.severity-calibration/v1",
    per_severity: (["high","medium","low"] | map(. as $s |
      ([$f[]|select(.severity==$s)]) as $rows |
      {
        severity: $s,
        findings: ($rows|length),
        corroborated: ([$rows[]|select(.corroborated)]|length),
        danger_rate: ((rate($rows)*1000|round)/1000)
      })),
    by_repository: ($f | group_by(.repo) | map(. as $g |
      {
        repo: $g[0].repo,
        ecosystem: $g[0].ecosystem,
        findings: ($g|length),
        high_rate: ((rate([$g[]|select(.severity=="high")])*1000|round)/1000),
        medium_rate: ((rate([$g[]|select(.severity=="medium")])*1000|round)/1000),
        low_rate: ((rate([$g[]|select(.severity=="low")])*1000|round)/1000),
        low_findings: ([$g[]|select(.severity=="low")]|length)
      })),
    summary: {
      findings: ($f|length),
      repositories: ($f|map(.repo)|unique|length),
      elevated_danger_rate: (($elevated_rate*1000|round)/1000),
      low_danger_rate: (($low_rate*1000|round)/1000),
      lift: ((($elevated_rate - $low_rate)*1000|round)/1000),
      severities_present: ([["high","medium","low"][] | select(. as $s | any($f[]; .severity==$s))] | length),
      calibrated: ($elevated_rate > $low_rate),
      verified: (($f|length) > 0 and ($elevated_rate > $low_rate))
    }
  }
' "$findings_jsonl" > "$OUT/severity-calibration.json"

{
  echo "# Calibrated severity validation"
  echo
  echo "Patchline severity is validated against independent danger evidence mined from each repository: incident/cause clusters, rollback/fix repair migrations, and recurring-hazard signals. A finding is *danger-corroborated* when the table it touches is referenced by that evidence."
  echo
  echo "## Calibration by severity"
  echo
  echo "| Severity | Findings | Danger-corroborated | Danger rate |"
  echo "| --- | ---: | ---: | ---: |"
  jq -r '.per_severity[] | "| " + .severity + " | " + (.findings|tostring) + " | " + (.corroborated|tostring) + " | " + (.danger_rate|tostring) + " |"' "$OUT/severity-calibration.json"
  echo
  echo "## Summary"
  jq -r '.summary | "- elevated (high+medium) danger rate: `" + (.elevated_danger_rate|tostring) + "`\n- low danger rate: `" + (.low_danger_rate|tostring) + "`\n- calibration lift (elevated - low): `" + (.lift|tostring) + "`"' "$OUT/severity-calibration.json"
  echo
  echo "## By repository"
  echo
  echo "| Repo | Ecosystem | Findings | High rate | Medium rate | Low rate |"
  echo "| --- | --- | ---: | ---: | ---: | ---: |"
  jq -r '.by_repository[] | "| `" + .repo + "` | " + .ecosystem + " | " + (.findings|tostring) + " | " + (.high_rate|tostring) + " | " + (.medium_rate|tostring) + " | " + (.low_rate|tostring) + " |"' "$OUT/severity-calibration.json"
} > "$OUT/severity-calibration.md"

cp "$OUT/severity-calibration.md" "$OUT/README.md"
echo "severity calibration complete: findings $(jq '.summary.findings' "$OUT/severity-calibration.json"), lift $(jq '.summary.lift' "$OUT/severity-calibration.json"), calibrated $(jq '.summary.calibrated' "$OUT/severity-calibration.json")"
