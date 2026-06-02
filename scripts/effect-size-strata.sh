#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/effect-size-strata-gate.json}"
OUT="${2:-results/generated/effect-size-strata}"
rm -rf "$OUT"
mkdir -p "$OUT/cache" "$OUT/analyses"

jq -e '
  .version == "patchline.effect-size-strata-gate/v1" and
  (.claim | length) > 100 and
  (.slices | length) >= .minimum_slices and
  all(.slices[]; (.repo | contains("/")) and (.ref | test("^[0-9a-f]{40}$")) and (.ecosystem|length)>0 and (.size|length)>0 and (.framework|length)>0)
' "$SPEC" > /dev/null

file_samples="$OUT/file-samples.jsonl"   # one row per migration file (risk density sample)
risk_samples="$OUT/risk-samples.jsonl"   # one row per risk (score + hazard class sample)
: > "$file_samples"; : > "$risk_samples"

hazard_classes="$(jq -c '.hazard_classes' "$SPEC")"
slice_count="$(jq '.slices | length' "$SPEC")"
for ((s=0; s<slice_count; s++)); do
  repo="$(jq -r ".slices[$s].repo" "$SPEC")"
  ref="$(jq -r ".slices[$s].ref" "$SPEC")"
  subpath="$(jq -r ".slices[$s].subpath" "$SPEC")"
  eco="$(jq -r ".slices[$s].ecosystem" "$SPEC")"
  fw="$(jq -r ".slices[$s].framework" "$SPEC")"
  size="$(jq -r ".slices[$s].size" "$SPEC")"
  id="$(printf '%s' "${repo//\//-}" | tr -c 'A-Za-z0-9_.-' '-')"
  adir="$OUT/analyses/$id"
  mkdir -p "$adir"
  go run ./cmd/patchline repo analyze \
    --github "$repo" --ref "$ref" --subpath "$subpath" \
    --download-dir "$OUT/cache" --stages inventory,baseline --no-llm \
    --out "$adir" --json > "$OUT/analyze-$id.log"
  base="$adir/baseline/baseline.json"

  # Per-file risk density samples.
  jq -c --arg eco "$eco" --arg fw "$fw" --arg size "$size" '
    ((.risks // []) | group_by(.path) | map({path:.[0].path, risks:length, high:(map(select(.severity=="high"))|length)})) as $byfile |
    $byfile[] | {ecosystem:$eco, framework:$fw, size:$size, risks:.risks, high:.high}
  ' "$base" >> "$file_samples"

  # Per-risk score + hazard-class samples.
  jq -c --arg eco "$eco" --argjson hc "$hazard_classes" '
    (.risks // [])[] | .kind as $k |
    ([$hc | to_entries[] | select(.value as $pat | $k | test($pat)) | .key] | first // "other") as $cls |
    {ecosystem:$eco, hazard_class:$cls, score:.score, kind:$k}
  ' "$base" >> "$risk_samples"
done

# Cohen's d effect-size engine over the per-file (risk density) samples, grouped by dimension.
cohen_d='
  def stats(a): (a|length) as $n | (if $n>0 then (a|add)/$n else 0 end) as $m |
    (if $n>1 then (a|map((.-$m)*(.-$m))|add)/($n-1) else 0 end) as $v | {n:$n, mean:$m, var:$v};
  def d(a; b): stats(a) as $A | stats(b) as $B |
    (if ($A.n+$B.n-2) > 0 then ((($A.n-1)*$A.var + ($B.n-1)*$B.var)/($A.n+$B.n-2)) | sqrt else 0 end) as $sp |
    (if $sp > 0 then (($A.mean-$B.mean)/$sp) else 0 end);
  def pairs(groups; key):
    [ groups | keys[] ] as $ks |
    [ range(0;($ks|length)) as $i | range($i+1;($ks|length)) as $j |
      ($ks[$i]) as $a | ($ks[$j]) as $b |
      { pair: ($a + " vs " + $b),
        groupA: $a, groupB: $b,
        nA: (groups[$a]|length), nB: (groups[$b]|length),
        meanA: ((stats(groups[$a]).mean*1000|round)/1000),
        meanB: ((stats(groups[$b]).mean*1000|round)/1000),
        cohens_d: ((d(groups[$a]; groups[$b])*1000|round)/1000) } ];
'

jq -s --argjson hc "$(jq -c '.hazard_classes' "$SPEC")" "
  $cohen_d
  . as \$files |
  (\$files | group_by(.ecosystem) | map({key:.[0].ecosystem, value:[.[].risks]}) | from_entries) as \$by_eco |
  (\$files | group_by(.size)      | map({key:.[0].size,      value:[.[].risks]}) | from_entries) as \$by_size |
  (\$files | group_by(.framework) | map({key:.[0].framework, value:[.[].risks]}) | from_entries) as \$by_fw |
  def topabs(ps): (ps | sort_by(-(.cohens_d | if . < 0 then -. else . end)) | first);
  { ecosystem: pairs(\$by_eco; \"ecosystem\"),
    size:      pairs(\$by_size; \"size\"),
    framework: pairs(\$by_fw; \"framework\"),
    largest: {
      ecosystem: topabs(pairs(\$by_eco; \"ecosystem\")),
      size:      topabs(pairs(\$by_size; \"size\")),
      framework: topabs(pairs(\$by_fw; \"framework\"))
    },
    total_files: (\$files | length) }
" "$file_samples" > "$OUT/effect-density.json"

# Hazard-class effect sizes on per-risk score.
jq -s "
  $cohen_d
  . as \$risks |
  (\$risks | group_by(.hazard_class) | map({key:.[0].hazard_class, value:[.[].score]}) | from_entries) as \$by_cls |
  def topabs(ps): (ps | sort_by(-(.cohens_d | if . < 0 then -. else . end)) | first);
  { hazard_class: pairs(\$by_cls; \"hazard_class\"),
    largest: topabs(pairs(\$by_cls; \"hazard_class\")),
    class_means: (\$risks | group_by(.hazard_class) | map({class:.[0].hazard_class, n:length, mean_score:((map(.score)|add)/length*100|round/100)})),
    total_risks: (\$risks | length) }
" "$risk_samples" > "$OUT/effect-hazard.json"

jq -n \
  --slurpfile density "$OUT/effect-density.json" \
  --slurpfile hazard "$OUT/effect-hazard.json" '
  {
    version: "patchline.effect-size-strata/v1",
    total_files: $density[0].total_files,
    total_risks: $hazard[0].total_risks,
    density_effects: { ecosystem: $density[0].ecosystem, size: $density[0].size, framework: $density[0].framework },
    largest_density_effects: $density[0].largest,
    hazard_class_effects: $hazard[0].hazard_class,
    hazard_class_means: $hazard[0].class_means,
    largest_hazard_effect: $hazard[0].largest,
    dimensions_reported: ["ecosystem","size","framework","hazard-class"]
  }
' > "$OUT/effect-size.json"

{
  echo "# Effect-size reports (real downloaded baselines)"
  echo
  jq -r '"Per-file risk-density samples: `" + (.total_files|tostring) + "`; per-risk score samples: `" + (.total_risks|tostring) + "`."' "$OUT/effect-size.json"
  echo
  echo "## Largest standardized effects (Cohen's d)"
  echo
  echo "| Dimension | Comparison | mean A | mean B | Cohen's d |"
  echo "| --- | --- | ---: | ---: | ---: |"
  jq -r '.largest_density_effects | to_entries[] | "| " + .key + " | " + .value.pair + " | " + (.value.meanA|tostring) + " | " + (.value.meanB|tostring) + " | " + (.value.cohens_d|tostring) + " |"' "$OUT/effect-size.json"
  jq -r '.largest_hazard_effect | "| hazard-class | " + .pair + " | " + (.meanA|tostring) + " | " + (.meanB|tostring) + " | " + (.cohens_d|tostring) + " |"' "$OUT/effect-size.json"
  echo
  echo "## Hazard-class mean scores"
  jq -r '.hazard_class_means[] | "- " + .class + ": mean score `" + (.mean_score|tostring) + "` (n=" + (.n|tostring) + ")"' "$OUT/effect-size.json"
} > "$OUT/effect-size.md"

cp "$OUT/effect-size.md" "$OUT/README.md"
echo "effect-size complete: files $(jq '.total_files' "$OUT/effect-size.json"), risks $(jq '.total_risks' "$OUT/effect-size.json"), largest eco d $(jq '.largest_density_effects.ecosystem.cohens_d' "$OUT/effect-size.json"), largest hazard d $(jq '.largest_hazard_effect.cohens_d' "$OUT/effect-size.json")"
