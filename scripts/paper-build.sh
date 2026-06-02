#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/paper-build-gate.json}"
OUT="${2:-results/generated/paper-build}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.paper-build-gate/v1" and (.gates|length) >= 1' "$SPEC" > /dev/null

# Verify each manifest gate has a backing script + doc, and collect a short claim.
rows="[]"
while IFS= read -r g; do
  test -f "scripts/$g-gate.sh"
  test -f "docs/$g.md"
  title="$(grep -m1 '^# ' "docs/$g.md" | sed 's/^# //')"
  rows="$(jq --arg g "$g" --arg t "$title" '. + [{gate:$g, title:$t}]' <<<"$rows")"
done < <(jq -r '.gates[]' "$SPEC")

echo "$rows" | jq '{version:"patchline.paper-build/v1", rows: ., row_count: length}' > "$OUT/paper-build.json"

# Emit the LaTeX fragment: a capabilities table, a bar-style figure, an appendix, and
# artifact links into docs/.
TEX="$OUT/paper-tables.tex"
{
  echo "% Auto-generated from gate data by scripts/paper-build.sh -- do not edit by hand."
  echo "\\section*{Generated capability catalog}"
  echo "\\begin{table}[h]\\centering"
  echo "\\begin{tabular}{ll}"
  echo "\\hline Gate & Capability \\\\ \\hline"
  jq -r '.rows[] | "\\texttt{" + (.gate|gsub("_";"\\_")) + "} & " + (.title|gsub("&";"\\&")) + " \\\\"' "$OUT/paper-build.json"
  echo "\\hline"
  echo "\\end{tabular}"
  echo "\\caption{Capabilities generated from the live gate catalog (\\texttt{make <gate>-gate}).}"
  echo "\\end{table}"
  echo
  echo "\\section*{Catalog size figure}"
  n="$(jq -r '.row_count' "$OUT/paper-build.json")"
  echo "\\noindent Catalog entries in this build: \\textbf{$n}. \\\\"
  echo "\\rule{${n}0pt}{8pt} % bar length proportional to entry count"
  echo
  echo "\\section*{Appendix: artifact links}"
  echo "\\begin{itemize}"
  jq -r '.rows[] | "\\item \\texttt{docs/" + .gate + ".md} --- \\texttt{scripts/" + .gate + "-gate.sh}"' "$OUT/paper-build.json"
  echo "\\end{itemize}"
} > "$TEX"

# Wrap and compile to prove the generated LaTeX is build-ready.
WRAP="$OUT/paper-standalone.tex"
{
  echo "\\documentclass{article}"
  echo "\\usepackage[margin=1in]{geometry}"
  echo "\\begin{document}"
  echo "\\input{paper-tables.tex}"
  echo "\\end{document}"
} > "$WRAP"

if command -v pdflatex > /dev/null 2>&1; then
  ( cd "$OUT" && pdflatex -interaction=nonstopmode -halt-on-error paper-standalone.tex > pdflatex.log 2>&1 )
  test -s "$OUT/paper-standalone.pdf" && compiled=true || compiled=false
else
  compiled=skipped
fi

jq --arg c "$compiled" '. + {compiled: $c}' "$OUT/paper-build.json" > "$OUT/paper-build.tmp" && mv "$OUT/paper-build.tmp" "$OUT/paper-build.json"

cp "$TEX" "$OUT/paper-build.md.src" 2>/dev/null || true
{
  echo "# Paper build pipeline"
  echo
  echo "Generated LaTeX rows: $(jq -r .row_count "$OUT/paper-build.json")"
  echo
  echo "Compiled to PDF: $(jq -r .compiled "$OUT/paper-build.json")"
} > "$OUT/paper-build.md"
cp "$OUT/paper-build.md" "$OUT/README.md"

echo "paper-build worker: rows=$(jq -r .row_count "$OUT/paper-build.json") compiled=$(jq -r .compiled "$OUT/paper-build.json")"
