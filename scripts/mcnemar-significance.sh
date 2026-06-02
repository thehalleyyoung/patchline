#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/mcnemar-significance-gate.json}"; OUT="${2:-results/generated/mcnemar-significance}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.mcnemar-significance-gate/v1"' "$SPEC" > /dev/null
jq '
  def r4: (.*10000|round)/10000;
  def absf: if . < 0 then -. else . end;
  def pct($xs; $p): ($xs|sort) as $s | ($s|length) as $n | ($p*($n-1)) as $idx | ($idx|floor) as $lo
    | ($s[$lo]) + (($idx-$lo) * (($s[ ([$lo+1,$n-1]|min) ]) - ($s[$lo])));
  def analyze($g; $crit):
    ($g.b_only_patchline_correct) as $b | ($g.c_only_baseline_correct) as $c
    | (if ($b+$c) == 0 then 0 else (((($b-$c)|absf) - 1) | (.*.)) / ($b+$c) end) as $stat
    | ($g.resample_diffs) as $rs
    | (pct($rs; 0.025)) as $lo | (pct($rs; 0.975)) as $hi
    | {b:$b, c:$c, statistic: ($stat|r4), significant: ($stat > $crit), ci_low: ($lo|r4), ci_high: ($hi|r4), ci_excludes_zero: (($lo > 0) or ($hi < 0))};
  .chi2_crit_0_05 as $crit
  | {
      version: "patchline.mcnemar-significance/v1",
      improved: analyze(.improved; $crit),
      identical: analyze(.identical; $crit)
    }
' "$SPEC" > "$OUT/mcnemar.json"
{ echo "# McNemar significance + bootstrap CI"; echo; echo "Improved: stat=$(jq -r '.improved.statistic' "$OUT/mcnemar.json") sig=$(jq -r '.improved.significant' "$OUT/mcnemar.json") CI=[$(jq -r '.improved.ci_low' "$OUT/mcnemar.json"),$(jq -r '.improved.ci_high' "$OUT/mcnemar.json")]"; echo "Identical: stat=$(jq -r '.identical.statistic' "$OUT/mcnemar.json") sig=$(jq -r '.identical.significant' "$OUT/mcnemar.json")"; } > "$OUT/mcnemar.md"
cp "$OUT/mcnemar.md" "$OUT/README.md"
echo "mcnemar-significance worker: improved_sig=$(jq -r '.improved.significant' "$OUT/mcnemar.json") identical_sig=$(jq -r '.identical.significant' "$OUT/mcnemar.json")"
