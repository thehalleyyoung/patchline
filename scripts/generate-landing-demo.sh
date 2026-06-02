#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/landing-readme-gate.json}"
OUT="${2:-results/generated/landing-readme-demo}"
rm -rf "$OUT"
mkdir -p "$OUT/cache" "$OUT/analysis"

repo="$(jq -r '.repo' "$SPEC")"
ref="$(jq -r '.ref' "$SPEC")"
subpath="$(jq -r '.subpath' "$SPEC")"

go run ./cmd/patchline repo analyze \
  --github "$repo" \
  --ref "$ref" \
  --subpath "$subpath" \
  --download-dir "$OUT/cache" \
  --stages inventory,baseline,propose,compare \
  --proposal-kind all \
  --budget files=8,lines=100,tokens=12000,changes=2 \
  --no-llm \
  --out "$OUT/analysis" \
  --json > "$OUT/analyze-stdout.json"

jq -n \
  --arg repo "$repo" \
  --arg ref "$ref" \
  --arg subpath "$subpath" \
  --slurpfile analyze "$OUT/analysis/analyze.json" \
  '{
    version:"patchline.landing-demo/v1",
    repo:$repo,
    ref:$ref,
    subpath:$subpath,
    files_scanned:$analyze[0].summary.files_scanned,
    facts:$analyze[0].summary.facts,
    ranked_risks:$analyze[0].summary.ranked_risks,
    provenance_slices:$analyze[0].summary.provenance_slices,
    generated_files:$analyze[0].summary.generated_files,
    compare_checks_failed:$analyze[0].summary.compare_checks_failed,
    deterministic_only:$analyze[0].summary.deterministic_only,
    baseline_hash:$analyze[0].summary.baseline_hash,
    proposal_hash:$analyze[0].summary.proposal_hash,
    compare_hash:$analyze[0].summary.compare_hash
  }' > "$OUT/landing-demo.json"

files="$(jq '.files_scanned' "$OUT/landing-demo.json")"
risks="$(jq '.ranked_risks' "$OUT/landing-demo.json")"
provenance="$(jq '.provenance_slices' "$OUT/landing-demo.json")"
generated="$(jq '.generated_files' "$OUT/landing-demo.json")"
failed="$(jq '.compare_checks_failed' "$OUT/landing-demo.json")"

cat > "$OUT/landing-demo.md" <<EOF
# Landing demo output

Real pinned output from \`$repo\` \`$subpath\`:

| Metric | Value |
| --- | ---: |
| files scanned | $files |
| ranked data-change risks | $risks |
| provenance slices | $provenance |
| generated review artifacts | $generated |
| deterministic checks failed | $failed |
EOF

cat > "$OUT/landing-demo.svg" <<EOF
<svg xmlns="http://www.w3.org/2000/svg" width="980" height="430" viewBox="0 0 980 430" role="img" aria-label="Patchline landing demo output for $repo $subpath">
  <rect width="980" height="430" rx="18" fill="#0b1020"/>
  <rect x="18" y="18" width="944" height="394" rx="14" fill="#111827" stroke="#374151"/>
  <circle cx="45" cy="45" r="7" fill="#ef4444"/>
  <circle cx="68" cy="45" r="7" fill="#f59e0b"/>
  <circle cx="91" cy="45" r="7" fill="#22c55e"/>
  <text x="120" y="51" fill="#d1d5db" font-family="Menlo, Consolas, monospace" font-size="14">60-second Patchline demo on $repo $subpath</text>
  <text x="42" y="92" fill="#93c5fd" font-family="Menlo, Consolas, monospace" font-size="15">\$ go run ./cmd/patchline repo analyze --github $repo --subpath $subpath --no-llm --json</text>
  <text x="42" y="135" fill="#e5e7eb" font-family="Menlo, Consolas, monospace" font-size="18">files scanned</text>
  <text x="300" y="135" fill="#f9fafb" font-family="Menlo, Consolas, monospace" font-size="24" font-weight="700">$files</text>
  <text x="42" y="180" fill="#e5e7eb" font-family="Menlo, Consolas, monospace" font-size="18">ranked data-change risks</text>
  <text x="300" y="180" fill="#f9fafb" font-family="Menlo, Consolas, monospace" font-size="24" font-weight="700">$risks</text>
  <text x="42" y="225" fill="#e5e7eb" font-family="Menlo, Consolas, monospace" font-size="18">provenance slices</text>
  <text x="300" y="225" fill="#f9fafb" font-family="Menlo, Consolas, monospace" font-size="24" font-weight="700">$provenance</text>
  <text x="42" y="270" fill="#e5e7eb" font-family="Menlo, Consolas, monospace" font-size="18">generated review artifacts</text>
  <text x="300" y="270" fill="#f9fafb" font-family="Menlo, Consolas, monospace" font-size="24" font-weight="700">$generated</text>
  <text x="42" y="315" fill="#e5e7eb" font-family="Menlo, Consolas, monospace" font-size="18">deterministic checks failed</text>
  <text x="300" y="315" fill="#22c55e" font-family="Menlo, Consolas, monospace" font-size="24" font-weight="700">$failed</text>
  <text x="42" y="365" fill="#a7f3d0" font-family="Menlo, Consolas, monospace" font-size="16">Writes inventory, baseline, proposal, compare, figures, and reviewer artifacts.</text>
</svg>
EOF

echo "landing demo generated: files $files, risks $risks, generated $generated"
