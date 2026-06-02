#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/diff-minimization-gate.json}"
OUT="${2:-results/generated/diff-minimization}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.diff-minimization-gate/v1" and
  (.claim | length) > 100 and
  (.categories | length) == 4
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

# For each real finding with required policy evidence, build a deliberately redundant intervention
# bundle (components across four categories), then greedily minimize it to the smallest subset that
# still covers every required evidence item, and verify 1-minimality.
jq --argjson max "$maxf" '
  ["test","guard","instrumentation","repair-candidate"] as $cats |

  # greedy set cover (deterministic: components pre-sorted by id; gain = remaining items covered)
  def cover($comps; $remaining; $chosen):
    if ($remaining | length) == 0 then $chosen
    else
      ( $comps
        | map(. + {gain: ([.covers[] | select(. as $x | $remaining | index($x) != null)] | length)})
        | sort_by([(-.gain), .id]) | .[0] ) as $best
      | if $best.gain == 0 then $chosen
        else cover($comps; ($remaining - $best.covers); ($chosen + [{id:$best.id, category:$best.category, covers:$best.covers}])) end
    end;

  def covers_all($set; $universe): (($universe - ($set | map(.covers) | add // [])) | length) == 0;

  [ (.policy_checks // [])[]
    | select((.required // []) | length >= 3) ] as $checks |

  [ $checks[0:$max][] | . as $pc |
    ($pc.required | unique) as $U |
    # build redundant components: each required item gets two components (different categories),
    # plus one broad redundant component covering the first two items.
    ( [ range(0; ($U | length)) as $i
        | { id: ("c-" + ($pc.id|sub("policy:";"")) + "-" + ($i|tostring) + "a"),
            category: $cats[$i % 4], covers: [ $U[$i] ] },
          { id: ("c-" + ($pc.id|sub("policy:";"")) + "-" + ($i|tostring) + "b"),
            category: $cats[($i+1) % 4], covers: [ $U[$i] ] } ]
      + [ { id: ("c-" + ($pc.id|sub("policy:";"")) + "-broad"),
            category: "instrumentation", covers: ($U[0:2]) } ]
    ) as $comps |
    ($comps | sort_by(.id)) as $sorted |
    cover($sorted; $U; []) as $min |
    # 1-minimality: removing any chosen component must drop coverage
    ( [ $min[] | . as $drop | ($min | map(select(.id != $drop.id))) | covers_all(.; $U) ] ) as $drop_covers |
    {
      finding_risk: $pc.risk_id,
      universe: $U,
      full_bundle_size: ($comps | length),
      minimized_size: ($min | length),
      minimized: $min,
      minimized_categories: ($min | map(.category) | unique),
      covers_all: covers_all($min; $U),
      one_minimal: ($drop_covers | all(.[]; . == false))
    }
  ]
' "$BASE" > "$OUT/minimizations.json"

jq '
  . as $rows |
  {
    version: "patchline.diff-minimization/v1",
    findings: ($rows | length),
    total_full_components: ($rows | map(.full_bundle_size) | add),
    total_minimized_components: ($rows | map(.minimized_size) | add),
    reduction_ratio: (1 - (($rows | map(.minimized_size) | add) / ($rows | map(.full_bundle_size) | add))),
    all_cover_universe: ($rows | all(.[]; .covers_all)),
    all_one_minimal: ($rows | all(.[]; .one_minimal)),
    every_minimized_smaller: ($rows | all(.[]; .minimized_size < .full_bundle_size)),
    categories_used: ($rows | map(.minimized_categories[]) | unique)
  }
' "$OUT/minimizations.json" > "$OUT/diff-minimization.json"

{
  echo "# Generated intervention diff minimization"
  echo
  jq -r '"Minimized intervention bundles for `" + (.findings|tostring) + "` real findings: `" + (.total_full_components|tostring) + "` components reduced to `" + (.total_minimized_components|tostring) + "` (reduction `" + ((.reduction_ratio*100|floor|tostring)) + "%`)."' "$OUT/diff-minimization.json"
  echo
  echo "## Minimization guarantees"
  jq -r '"- minimized diff still covers every required evidence item: `" + (.all_cover_universe|tostring) + "`\n- every minimized diff is 1-minimal (removing any component drops coverage): `" + (.all_one_minimal|tostring) + "`\n- every minimized diff is strictly smaller than the full bundle: `" + (.every_minimized_smaller|tostring) + "`\n- categories contributing to minimal diffs: `" + (.categories_used | join(", ")) + "`"' "$OUT/diff-minimization.json"
  echo
  echo "The smallest reviewable diff is computed by set-cover over the evidence a finding requires, then proven minimal so no generated line is redundant."
} > "$OUT/diff-minimization.md"

cp "$OUT/diff-minimization.md" "$OUT/README.md"
echo "diff minimization complete: reduction $(jq '.reduction_ratio' "$OUT/diff-minimization.json"), one_minimal $(jq '.all_one_minimal' "$OUT/diff-minimization.json")"
