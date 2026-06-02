#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/docs-site-gate.json}"
OUT="${2:-results/generated/docs-site}"
SITE="$OUT/site"
rm -rf "$OUT"
mkdir -p "$SITE/tutorials" "$SITE/artifacts" "$OUT/cache"

jq -e '
  .version == "patchline.docs-site-gate/v1" and
  (.claim | length) > 150 and
  (.site_url | test("^https://")) and
  (.required_roles | length) == 4 and
  (.required_pages | length) >= 8 and
  (.real_code.repo | length) > 0 and
  (.real_code.ref | test("^[0-9a-f]{40}$")) and
  (.real_code.subpath | length) > 0
' "$SPEC" > /dev/null

repo="$(jq -r '.real_code.repo' "$SPEC")"
ref="$(jq -r '.real_code.ref' "$SPEC")"
subpath="$(jq -r '.real_code.subpath' "$SPEC")"
site_url="$(jq -r '.site_url' "$SPEC")"

go run ./cmd/patchline repo analyze \
  --github "$repo" \
  --ref "$ref" \
  --subpath "$subpath" \
  --download-dir "$OUT/cache" \
  --stages inventory,baseline,propose,compare \
  --proposal-kind all \
  --budget files=6,lines=90,tokens=10000,changes=2 \
  --no-llm \
  --out "$OUT/analysis" \
  --json > "$OUT/analyze-stdout.json"

jq -n \
  --arg repo "$repo" \
  --arg ref "$ref" \
  --arg subpath "$subpath" \
  --slurpfile analyze "$OUT/analysis/analyze.json" \
  '{
    version:"patchline.docs-public-demo/v1",
    repo:$repo,
    ref:$ref,
    subpath:$subpath,
    files_scanned:$analyze[0].summary.files_scanned,
    ranked_risks:$analyze[0].summary.ranked_risks,
    provenance_slices:$analyze[0].summary.provenance_slices,
    generated_files:$analyze[0].summary.generated_files,
    compare_checks_failed:$analyze[0].summary.compare_checks_failed,
    deterministic_only:$analyze[0].summary.deterministic_only
  }' > "$SITE/artifacts/public-demo.json"

files="$(jq '.files_scanned' "$SITE/artifacts/public-demo.json")"
risks="$(jq '.ranked_risks' "$SITE/artifacts/public-demo.json")"
provenance="$(jq '.provenance_slices' "$SITE/artifacts/public-demo.json")"
generated="$(jq '.generated_files' "$SITE/artifacts/public-demo.json")"
failed="$(jq '.compare_checks_failed' "$SITE/artifacts/public-demo.json")"

cat > "$SITE/styles.css" <<'CSS'
:root { color-scheme: light; font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
body { margin: 0; color: #172033; background: #f8fafc; }
header { background: #0f172a; color: #fff; padding: 48px 32px; }
main { max-width: 1040px; margin: 0 auto; padding: 32px; }
nav a { color: #bfdbfe; margin-right: 16px; font-weight: 700; }
.card { background: #fff; border: 1px solid #e5e7eb; border-radius: 14px; padding: 24px; margin: 18px 0; box-shadow: 0 1px 2px rgba(15, 23, 42, .06); }
.grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 16px; }
.metric { font-size: 32px; font-weight: 800; color: #2563eb; }
code, pre { background: #111827; color: #e5e7eb; border-radius: 8px; padding: 2px 6px; }
pre { padding: 16px; overflow-x: auto; }
footer { color: #64748b; padding: 32px; text-align: center; }
CSS

nav='<nav><a href="../index.html">Home</a><a href="maintainers.html">Maintainers</a><a href="researchers.html">Researchers</a><a href="security-reviewers.html">Security reviewers</a><a href="contributors.html">Contributors</a></nav>'

cat > "$SITE/index.html" <<EOF
<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Patchline documentation</title><link rel="stylesheet" href="styles.css"></head>
<body>
<header><h1>Patchline documentation</h1><p>Deterministic data-change repair evidence for maintainers, researchers, security reviewers, and contributors.</p><nav><a href="tutorials/maintainers.html">Maintainers</a><a href="tutorials/researchers.html">Researchers</a><a href="tutorials/security-reviewers.html">Security reviewers</a><a href="tutorials/contributors.html">Contributors</a></nav></header>
<main>
<section class="card"><h2>Real public-repo output</h2><p>This site build regenerated <code>$repo</code> <code>$subpath</code> at <code>$ref</code>.</p><div class="grid"><div><div class="metric">$files</div><p>files scanned</p></div><div><div class="metric">$risks</div><p>ranked risks</p></div><div><div class="metric">$provenance</div><p>provenance slices</p></div><div><div class="metric">$generated</div><p>generated review artifacts</p></div><div><div class="metric">$failed</div><p>deterministic checks failed</p></div></div><p><a href="artifacts/public-demo.json">Download the public demo JSON</a>.</p></section>
<section class="card"><h2>Start by role</h2><ul><li><a href="tutorials/maintainers.html">Maintainers: first run, triage, PR comments, and reviewer walkthroughs.</a></li><li><a href="tutorials/researchers.html">Researchers: claims, figures, limitations, and reproducible artifacts.</a></li><li><a href="tutorials/security-reviewers.html">Security reviewers: threat model, quarantine, release checksums, and offline mode.</a></li><li><a href="tutorials/contributors.html">Contributors: local checks, fixtures, fuzzing, plugins, and issue templates.</a></li></ul></section>
</main><footer>Generated by <code>scripts/build-docs-site.sh</code>.</footer></body></html>
EOF

cat > "$SITE/tutorials/maintainers.html" <<EOF
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Maintainer tutorial - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><h1>Maintainer tutorial</h1>$nav</header><main><section class="card"><h2>First-run analysis</h2><pre>go run ./cmd/patchline repo analyze --github $repo --ref $ref --subpath $subpath --stages inventory,baseline,propose,compare --no-llm --out results/patchline-demo</pre><p>Review <code>baseline/baseline.md</code>, <code>compare/compare.md</code>, and generated artifacts under <code>proposal/patchline-proposals/</code>.</p></section><section class="card"><h2>Triage and PR review</h2><p>Use <code>repo pr-comment</code>, CODEOWNERS routing, and <code>repo hook pre-push</code> to keep risky data changes visible before merge.</p></section><section class="card"><h2>Reviewer walkthrough</h2><p>Run <code>make reviewer-walkthrough-gate</code> to regenerate tables, figures, reports, and case-study bundles.</p></section></main></body></html>
EOF

cat > "$SITE/tutorials/researchers.html" <<'EOF'
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Researcher tutorial - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><h1>Researcher tutorial</h1><nav><a href="../index.html">Home</a><a href="maintainers.html">Maintainers</a><a href="researchers.html">Researchers</a><a href="security-reviewers.html">Security reviewers</a><a href="contributors.html">Contributors</a></nav></header><main><section class="card"><h2>Paper claims</h2><p>Use <code>repo claims-evidence</code> to map abstract, introduction, and evaluation claims to artifacts, limitations, missing evidence, and reviewer checks.</p></section><section class="card"><h2>Figures and tables</h2><p>Use <code>repo figures</code>, <code>repo metrics</code>, and <code>repo taxonomy</code> to regenerate paper-ready SVG/JSON figures and evaluation tables.</p></section><section class="card"><h2>Reproducibility</h2><p>Use <code>reviewer-walkthrough-gate</code>, release checksums, and pinned public refs to keep paper artifacts auditable.</p></section></main></body></html>
EOF

cat > "$SITE/tutorials/security-reviewers.html" <<'EOF'
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Security reviewer tutorial - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><h1>Security reviewer tutorial</h1><nav><a href="../index.html">Home</a><a href="maintainers.html">Maintainers</a><a href="researchers.html">Researchers</a><a href="security-reviewers.html">Security reviewers</a><a href="contributors.html">Contributors</a></nav></header><main><section class="card"><h2>Threat model</h2><p>Start with <code>docs/threat-model.md</code> and <code>make threat-model-gate</code> for untrusted repos, archives, generated code, native checks, and adapter inputs.</p></section><section class="card"><h2>Generated-code quarantine</h2><p>Generated artifacts remain untrusted, non-executable review material until deterministic compare and maintainer evidence say otherwise.</p></section><section class="card"><h2>Release integrity</h2><p>Use signed release checksums, supply-chain provenance, and offline validation before trusting release or CI artifacts.</p></section></main></body></html>
EOF

cat > "$SITE/tutorials/contributors.html" <<'EOF'
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Contributor tutorial - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><h1>Contributor tutorial</h1><nav><a href="../index.html">Home</a><a href="maintainers.html">Maintainers</a><a href="researchers.html">Researchers</a><a href="security-reviewers.html">Security reviewers</a><a href="contributors.html">Contributors</a></nav></header><main><section class="card"><h2>Local checks</h2><p>Run <code>patchline contributor check</code> or the focused gate for your feature before opening a PR.</p></section><section class="card"><h2>Real-repo fixtures</h2><p>Use <code>golden-fixture generate</code> and public issue templates to add small, reproducible examples without vendoring whole repositories.</p></section><section class="card"><h2>Extension points</h2><p>Use <code>plugins list</code>, parser/fact/ranker interfaces, fuzz seeds, and compatibility gates to add ecosystems safely.</p></section></main></body></html>
EOF

touch "$SITE/.nojekyll"
cat > "$SITE/sitemap.xml" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>${site_url}</loc></url>
  <url><loc>${site_url}tutorials/maintainers.html</loc></url>
  <url><loc>${site_url}tutorials/researchers.html</loc></url>
  <url><loc>${site_url}tutorials/security-reviewers.html</loc></url>
  <url><loc>${site_url}tutorials/contributors.html</loc></url>
</urlset>
EOF

jq -n \
  --arg site_url "$site_url" \
  --arg repo "$repo" \
  --arg ref "$ref" \
  --arg subpath "$subpath" \
  --argjson pages "$(find "$SITE" -type f | sed "s#^$SITE/##" | sort | jq -R . | jq -s .)" \
  --slurpfile demo "$SITE/artifacts/public-demo.json" \
  '{
    version:"patchline.docs-site/v1",
    site_url:$site_url,
    public_demo:{repo:$repo, ref:$ref, subpath:$subpath, summary:$demo[0]},
    roles:["maintainers","researchers","security-reviewers","contributors"],
    pages:$pages
  }' > "$SITE/site-manifest.json"

echo "docs site built: $SITE pages $(jq '.pages | length' "$SITE/site-manifest.json") risks $risks"
