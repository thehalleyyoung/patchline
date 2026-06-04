#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/docs-site-gate.json}"
OUT="${2:-results/generated/docs-site}"
SITE="$OUT/site"
LOCAL="$OUT/local-checks"
rm -rf "$OUT"
mkdir -p "$SITE/tutorials" "$SITE/scenarios" "$SITE/reference" "$SITE/artifacts" "$OUT/cache" "$OUT/commands" "$LOCAL"

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

command_rows=()
run_doc_command() {
  local id="$1"
  local title="$2"
  local command="$3"
  local stdout="$OUT/commands/$id.stdout"
  local stderr="$OUT/commands/$id.stderr"
  local row="$OUT/commands/$id.json"
  set +e
  bash -lc "$command" > "$stdout" 2> "$stderr"
  local status="$?"
  set -e
  jq -n \
    --arg id "$id" \
    --arg title "$title" \
    --arg command "$command" \
    --arg stdout "commands/$id.stdout" \
    --arg stderr "commands/$id.stderr" \
    --argjson exit_code "$status" \
    '{
      id:$id,
      title:$title,
      command:$command,
      exit_code:$exit_code,
      success:($exit_code == 0),
      stdout:$stdout,
      stderr:$stderr
    }' > "$row"
  command_rows+=("$row")
  if [[ "$status" -ne 0 ]]; then
    echo "docs command failed: $id" >&2
    sed -n '1,120p' "$stderr" >&2
    return "$status"
  fi
}

run_doc_command about "Explain what Patchline is" \
  "go run ./cmd/patchline about"
run_doc_command doctor "Inspect the demo workspace" \
  "go run ./cmd/patchline doctor demos/billing --out $LOCAL/doctor"
run_doc_command intake "Extract local repair evidence" \
  "go run ./cmd/patchline intake demos/billing --out $LOCAL/intake"
run_doc_command inventory "Inventory files, facts, and commands" \
  "go run ./cmd/patchline repo inventory demos/billing --out $LOCAL/inventory --json"
run_doc_command baseline "Rank local data-change risks" \
  "go run ./cmd/patchline repo baseline --inventory $LOCAL/inventory --intake $LOCAL/intake --out $LOCAL/baseline --json"
run_doc_command propose "Generate quarantined review artifacts" \
  "go run ./cmd/patchline repo propose --from-report $LOCAL/baseline --proposal-kind all --budget files=3,lines=80,tokens=8000,changes=2 --no-llm --out $LOCAL/proposal --json"
run_doc_command compare "Re-check generated artifacts" \
  "go run ./cmd/patchline repo compare --before $LOCAL/baseline --after $LOCAL/proposal --out $LOCAL/compare --json"
run_doc_command db-semantics "Analyze SQL semantics" \
  "go run ./cmd/patchline db-semantics --engine postgres --sql demos/billing/migrations/002_bad_backfill.sql --out $LOCAL/db-semantics.json"
run_doc_command plugins-list "List extension points" \
  "go run ./cmd/patchline plugins list"
run_doc_command plugins-probe "Probe plugin coverage" \
  "go run ./cmd/patchline plugins probe demos/billing --out $LOCAL/plugins-probe"

for required in \
  "$LOCAL/doctor/doctor.json" \
  "$LOCAL/intake/summary.json" \
  "$LOCAL/inventory/inventory.json" \
  "$LOCAL/inventory/facts.jsonl" \
  "$LOCAL/baseline/baseline.json" \
  "$LOCAL/proposal/proposal.json" \
  "$LOCAL/proposal/proposal.patch" \
  "$LOCAL/compare/compare.json" \
  "$LOCAL/db-semantics.json" \
  "$LOCAL/plugins-probe/plugin-probe.json"; do
  test -s "$required"
done

jq -s \
  --arg local_root "$LOCAL" \
  '{
    version:"patchline.docs-command-results/v1",
    local_root:$local_root,
    commands:.,
    summary:{
      total:length,
      succeeded:(map(select(.success == true)) | length),
      failed:(map(select(.success == false)) | length)
    }
  }' "${command_rows[@]}" > "$SITE/artifacts/command-results.json"

local_files="$(jq '.files_scanned' "$LOCAL/inventory/inventory.json")"
local_facts="$(wc -l < "$LOCAL/inventory/facts.jsonl" | tr -d ' ')"
local_risks="$(jq '.risks | length' "$LOCAL/baseline/baseline.json")"
local_generated="$(jq '.generated_files | length' "$LOCAL/proposal/proposal.json")"
local_failed="$(jq '.summary.patchline_checks_failed' "$LOCAL/compare/compare.json")"

cat > "$SITE/styles.css" <<'CSS'
:root { color-scheme: light; font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; --ink:#172033; --muted:#64748b; --blue:#2563eb; --green:#059669; --bg:#f8fafc; --card:#fff; }
* { box-sizing: border-box; }
body { margin: 0; color: var(--ink); background: var(--bg); line-height: 1.6; }
header { background: radial-gradient(circle at 20% 20%, #1d4ed8 0, #0f172a 34rem); color: #fff; padding: 56px 32px; }
main { max-width: 1120px; margin: 0 auto; padding: 32px; }
nav a { color: #bfdbfe; margin-right: 16px; font-weight: 800; text-decoration: none; }
nav a:hover { text-decoration: underline; }
.eyebrow { color: #93c5fd; font-size: 13px; font-weight: 900; letter-spacing: .12em; text-transform: uppercase; }
.hero { max-width: 1060px; margin: 0 auto; }
.hero h1 { font-size: clamp(38px, 6vw, 74px); line-height: .96; margin: 12px 0 18px; letter-spacing: -.06em; }
.hero p { font-size: 20px; max-width: 820px; color: #dbeafe; }
.card { background: var(--card); border: 1px solid #e5e7eb; border-radius: 18px; padding: 26px; margin: 20px 0; box-shadow: 0 12px 28px rgba(15, 23, 42, .07); }
.grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(230px, 1fr)); gap: 16px; }
.metric { font-size: 34px; font-weight: 900; color: var(--blue); line-height: 1; }
.pill { display:inline-block; border:1px solid #bfdbfe; background:#eff6ff; color:#1d4ed8; border-radius:999px; padding:4px 10px; font-weight:800; font-size:12px; margin:2px; }
.ok { color: var(--green); font-weight: 900; }
code, pre { background: #111827; color: #e5e7eb; border-radius: 8px; padding: 2px 6px; }
pre { padding: 16px; overflow-x: auto; white-space: pre-wrap; }
table { width: 100%; border-collapse: collapse; }
th, td { border-bottom: 1px solid #e5e7eb; text-align: left; padding: 10px; vertical-align: top; }
footer { color: var(--muted); padding: 32px; text-align: center; }
CSS

root_nav='<nav><a href="index.html">Home</a><a href="quickstart.html">Quickstart</a><a href="architecture.html">Architecture</a><a href="scenarios/local-analysis.html">Scenarios</a><a href="reference/commands.html">Command proof</a></nav>'
tutorial_nav='<nav><a href="../index.html">Home</a><a href="../quickstart.html">Quickstart</a><a href="maintainers.html">Maintainers</a><a href="researchers.html">Researchers</a><a href="security-reviewers.html">Security reviewers</a><a href="contributors.html">Contributors</a></nav>'
scenario_nav='<nav><a href="../index.html">Home</a><a href="../quickstart.html">Quickstart</a><a href="local-analysis.html">Local</a><a href="public-repositories.html">Public repos</a><a href="generated-interventions.html">Generated review</a><a href="database-semantics.html">DB semantics</a><a href="ci-offline.html">CI/offline</a></nav>'
reference_nav='<nav><a href="../index.html">Home</a><a href="../architecture.html">Architecture</a><a href="commands.html">Commands</a><a href="artifacts.html">Artifacts</a><a href="validation.html">Validation</a></nav>'

cat > "$SITE/index.html" <<EOF
<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Patchline documentation</title><link rel="stylesheet" href="styles.css"></head>
<body>
<header><div class="hero"><div class="eyebrow">Verified GitHub Pages documentation</div><h1>Patchline turns data-change risk into reproducible evidence.</h1><p>Deterministic analysis for maintainers, researchers, security reviewers, and contributors: fetch real repositories, inventory facts, rank risks, generate bounded review artifacts, and re-check every generated artifact before review.</p>$root_nav</div></header>
<main>
<section class="card"><h2>Real public-repo output</h2><p>This site build regenerated <code>$repo</code> <code>$subpath</code> at pinned commit <code>$ref</code>.</p><div class="grid"><div><div class="metric">$files</div><p>public files scanned</p></div><div><div class="metric">$risks</div><p>public ranked risks</p></div><div><div class="metric">$provenance</div><p>provenance slices</p></div><div><div class="metric">$generated</div><p>generated review artifacts</p></div><div><div class="metric">$failed</div><p>deterministic checks failed</p></div></div><p><a href="artifacts/public-demo.json">Download public-demo.json</a> and <a href="artifacts/command-results.json">command-results.json</a>.</p></section>
<section class="card"><h2>Local command proof</h2><p>The docs build also ran the local demo workflow on <code>demos/billing</code>: <span class="pill">$local_files files</span><span class="pill">$local_facts facts</span><span class="pill">$local_risks ranked risks</span><span class="pill">$local_generated generated files</span><span class="pill">$local_failed failed checks</span>. Those numbers come from the generated JSON artifacts, not hand-written copy.</p></section>
<section class="card"><h2>Choose your path</h2><div class="grid"><div><h3>Run it</h3><p><a href="quickstart.html">Build and run the verified quickstart</a>, then inspect the artifact bundle.</p></div><div><h3>Understand it</h3><p><a href="architecture.html">Follow the fetch to compare pipeline</a> and the evidence each layer emits.</p></div><div><h3>Use it</h3><p><a href="scenarios/local-analysis.html">Scan local code</a>, <a href="scenarios/public-repositories.html">validate public repos</a>, and <a href="scenarios/ci-offline.html">wire CI/offline checks</a>.</p></div><div><h3>Audit it</h3><p><a href="reference/validation.html">Re-run the gates</a> and check every documented command sample.</p></div></div></section>
<section class="card"><h2>Start by role</h2><ul><li><a href="tutorials/maintainers.html">Maintainers: first run, triage, generated review artifacts, and reviewer handoff.</a></li><li><a href="tutorials/researchers.html">Researchers: claims-to-evidence maps, public corpora, limitations, and reproducible artifacts.</a></li><li><a href="tutorials/security-reviewers.html">Security reviewers: threat model, generated-code quarantine, release checksums, and offline mode.</a></li><li><a href="tutorials/contributors.html">Contributors: local checks, fixtures, plugins, and focused gates.</a></li></ul></section>
</main><footer>Generated by <code>scripts/build-docs-site.sh</code>.</footer></body></html>
EOF

cat > "$SITE/quickstart.html" <<EOF
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Patchline quickstart</title><link rel="stylesheet" href="styles.css"></head><body><header><div class="hero"><div class="eyebrow">Quickstart</div><h1>From checkout to evidence bundle.</h1><p>These are the stable local commands the site build ran against <code>demos/billing</code>.</p>$root_nav</div></header><main><section class="card"><h2>Build and inspect</h2><pre>go build -o bin/patchline ./cmd/patchline
bin/patchline about
bin/patchline doctor demos/billing --out results/generated/docs-local/doctor</pre><p>The docs build verifies the same command family with <code>go run ./cmd/patchline</code> and writes command status to <a href="artifacts/command-results.json">command-results.json</a>.</p></section><section class="card"><h2>Run the local pipeline</h2><pre>bin/patchline intake demos/billing --out results/generated/docs-local/intake
bin/patchline repo inventory demos/billing --out results/generated/docs-local/inventory --json
bin/patchline repo baseline --inventory results/generated/docs-local/inventory --intake results/generated/docs-local/intake --out results/generated/docs-local/baseline --json
bin/patchline repo propose --from-report results/generated/docs-local/baseline --proposal-kind all --budget files=3,lines=80,tokens=8000,changes=2 --no-llm --out results/generated/docs-local/proposal --json
bin/patchline repo compare --before results/generated/docs-local/baseline --after results/generated/docs-local/proposal --out results/generated/docs-local/compare --json</pre></section><section class="card"><h2>What you should see</h2><p>For the committed demo fixture, this build observed <strong>$local_files</strong> files, <strong>$local_facts</strong> facts, <strong>$local_risks</strong> ranked risks, <strong>$local_generated</strong> generated review files, and <strong>$local_failed</strong> Patchline compare failures.</p></section></main></body></html>
EOF

cat > "$SITE/architecture.html" <<EOF
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Patchline architecture</title><link rel="stylesheet" href="styles.css"></head><body><header><div class="hero"><div class="eyebrow">Architecture</div><h1>File-backed analysis from source to review.</h1><p>Patchline is deliberately boring infrastructure: every stage writes artifacts that another command can inspect, resume, redact, compare, or upload.</p>$root_nav</div></header><main><section class="card"><h2>Pipeline</h2><pre>source or evidence export
  -> fetch
  -> inventory
  -> intake
  -> baseline
  -> propose
  -> compare
  -> CI gate / offline replay</pre></section><section class="card"><h2>Layer outputs</h2><table><tr><th>Layer</th><th>What it proves locally</th></tr><tr><td>Inventory</td><td>Files, languages, migration roots, native commands, CODEOWNERS hints, and <code>facts.jsonl</code>.</td></tr><tr><td>Intake</td><td>Problem/cause/repair candidates, SARIF, evidence links, and time signals.</td></tr><tr><td>Baseline</td><td>Ranked risks, provenance slices, policy checks, symbolic obligations, recurrence, and proof holes.</td></tr><tr><td>Proposal</td><td>Bounded, quarantined generated artifacts plus prompt/context/output hashes.</td></tr><tr><td>Compare</td><td>Deterministic re-analysis of generated artifacts and optional native test results.</td></tr></table></section><section class="card"><h2>Design constraints</h2><p><span class="pill">deterministic by default</span><span class="pill">generated code quarantined</span><span class="pill">public repos pinned by commit</span><span class="pill">offline cache validation</span><span class="pill">claims tied to artifacts</span></p></section></main></body></html>
EOF

cat > "$SITE/tutorials/maintainers.html" <<EOF
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Maintainer tutorial - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">Maintainers</div><h1>Turn a risky migration review into a bounded evidence packet.</h1>$tutorial_nav</div></header><main><section class="card"><h2>First-run analysis</h2><pre>go run ./cmd/patchline repo analyze --github $repo --ref $ref --subpath $subpath --stages inventory,baseline,propose,compare --no-llm --out results/patchline-demo</pre><p>This site build ran that public shape and observed <strong>$risks</strong> ranked public risks with <strong>$generated</strong> generated review artifacts. Review <code>baseline/baseline.md</code>, <code>proposal/proposal.md</code>, and <code>compare/compare.md</code>.</p></section><section class="card"><h2>Daily workflow</h2><ol><li>Run local inventory/intake on the changed slice.</li><li>Read the top ranked risks and linked evidence.</li><li>Generate bounded tests, guards, or instrumentation with <code>--no-llm</code> or a local <code>--llm-command</code>.</li><li>Trust only artifacts that pass compare and human review.</li></ol></section><section class="card"><h2>Review handoff</h2><p>Attach the analysis bundle, SARIF, and generated patch as review evidence. Use <a href="../reference/artifacts.html">the artifact reference</a> to explain what each file means.</p></section></main></body></html>
EOF

cat > "$SITE/tutorials/researchers.html" <<'EOF'
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Researcher tutorial - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">Researchers</div><h1>Write claims that point to artifacts.</h1><nav><a href="../index.html">Home</a><a href="../reference/validation.html">Validation</a><a href="maintainers.html">Maintainers</a><a href="researchers.html">Researchers</a><a href="security-reviewers.html">Security reviewers</a><a href="contributors.html">Contributors</a></nav></div></header><main><section class="card"><h2>Claims-to-evidence discipline</h2><p>Map abstract, introduction, and evaluation claims to concrete artifacts, limitations, missing evidence, paper wording, reviewer checks, and affected repositories.</p></section><section class="card"><h2>Reproducible public slices</h2><p>Public examples should be pinned by repository, commit, and subpath. This site uses the same pattern for its embedded public demo and exposes the generated JSON for inspection.</p></section><section class="card"><h2>Artifact path</h2><p>The validation reference lists the exact gates used to keep benchmark, public-repo, and documentation claims synchronized.</p></section></main></body></html>
EOF

cat > "$SITE/tutorials/security-reviewers.html" <<'EOF'
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Security reviewer tutorial - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">Security reviewers</div><h1>Assume the repo, archive, and generated patch are untrusted.</h1><nav><a href="../index.html">Home</a><a href="../scenarios/generated-interventions.html">Quarantine</a><a href="../scenarios/ci-offline.html">Offline</a><a href="maintainers.html">Maintainers</a><a href="researchers.html">Researchers</a><a href="security-reviewers.html">Security reviewers</a><a href="contributors.html">Contributors</a></nav></div></header><main><section class="card"><h2>Threat model</h2><p>Patchline treats fetched repositories, generated interventions, evidence adapters, and native project commands as untrusted inputs. Generated artifacts are review files, not applied changes.</p></section><section class="card"><h2>Controls to verify</h2><ul><li>Content-addressed archive metadata in <code>source.json</code>.</li><li>Redaction before sharing bundles outside a trusted boundary.</li><li>Native tests skipped unless explicitly requested with <code>--run-native-tests</code>.</li><li>Compare reports for new risky SQL and failed Patchline checks.</li></ul></section><section class="card"><h2>Release integrity</h2><p>Use signed release checksums, supply-chain provenance gates, and offline validation before trusting release or CI artifacts.</p></section></main></body></html>
EOF

cat > "$SITE/tutorials/contributors.html" <<'EOF'
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Contributor tutorial - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">Contributors</div><h1>Add capabilities without breaking evidence.</h1><nav><a href="../index.html">Home</a><a href="../reference/commands.html">Commands</a><a href="../reference/validation.html">Validation</a><a href="maintainers.html">Maintainers</a><a href="researchers.html">Researchers</a><a href="security-reviewers.html">Security reviewers</a><a href="contributors.html">Contributors</a></nav></div></header><main><section class="card"><h2>Local checks</h2><p>Run the repository tests and the focused gate for your feature before opening a PR. Documentation changes should still pass the docs-site gate.</p></section><section class="card"><h2>Fixtures and plugins</h2><p>Use the verified plugin commands, parser/fact/ranker interfaces, golden fixtures, fuzz seeds, and compatibility gates to add ecosystems safely.</p></section><section class="card"><h2>Documentation standard</h2><p>If you document a command, add it to the command-results artifact or to a focused make gate so future docs builds can prove it still runs.</p></section></main></body></html>
EOF

cat > "$SITE/scenarios/local-analysis.html" <<EOF
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Local analysis scenario - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">Scenario</div><h1>Scan the repo or migration folder you already have.</h1>$scenario_nav</div></header><main><section class="card"><h2>Verified local flow</h2><pre>go run ./cmd/patchline doctor demos/billing --out $LOCAL/doctor
go run ./cmd/patchline intake demos/billing --out $LOCAL/intake
go run ./cmd/patchline repo inventory demos/billing --out $LOCAL/inventory --json
go run ./cmd/patchline repo baseline --inventory $LOCAL/inventory --intake $LOCAL/intake --out $LOCAL/baseline --json</pre><p>The generated proof for those commands is in <a href="../artifacts/command-results.json">command-results.json</a>. The current local fixture produced <strong>$local_risks</strong> ranked risks from <strong>$local_facts</strong> facts.</p></section><section class="card"><h2>When to use it</h2><p>Use local analysis for migration folders, exported evidence bundles, or a checked-out service before you spend time wiring CI.</p></section></main></body></html>
EOF

cat > "$SITE/scenarios/public-repositories.html" <<EOF
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Public repository scenario - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">Scenario</div><h1>Validate behavior on real public code.</h1>$scenario_nav</div></header><main><section class="card"><h2>Pinned public demo</h2><pre>go run ./cmd/patchline repo analyze --github $repo --ref $ref --subpath $subpath --stages inventory,baseline,propose,compare --proposal-kind all --budget files=6,lines=90,tokens=10000,changes=2 --no-llm --out results/generated/docs-site/analysis --json</pre><p>This exact site build regenerated the public demo with <strong>$files</strong> files, <strong>$risks</strong> ranked risks, and <strong>$generated</strong> generated review artifacts.</p></section><section class="card"><h2>Broader public matrix</h2><p>Use <code>make four-repo-demo</code> for the built-in public migration catalog, and keep any expanded public-repo claim pinned by repository, commit, and subpath.</p></section></main></body></html>
EOF

cat > "$SITE/scenarios/generated-interventions.html" <<EOF
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Generated interventions - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">Scenario</div><h1>Generate review artifacts, not unreviewed fixes.</h1>$scenario_nav</div></header><main><section class="card"><h2>Verified proposal and compare</h2><pre>go run ./cmd/patchline repo propose --from-report $LOCAL/baseline --proposal-kind all --budget files=3,lines=80,tokens=8000,changes=2 --no-llm --out $LOCAL/proposal --json
go run ./cmd/patchline repo compare --before $LOCAL/baseline --after $LOCAL/proposal --out $LOCAL/compare --json</pre><p>The local docs build produced <strong>$local_generated</strong> generated files and compare reported <strong>$local_failed</strong> Patchline check failures.</p></section><section class="card"><h2>Safety model</h2><p>Generated artifacts are non-executable review material. Patchline records budget, target risk IDs, prompt context, output hashes, quarantine metadata, and deterministic compare results before a human considers the artifact useful.</p></section></main></body></html>
EOF

cat > "$SITE/scenarios/database-semantics.html" <<EOF
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Database semantics - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">Scenario</div><h1>Look past SQL text into operational semantics.</h1>$scenario_nav</div></header><main><section class="card"><h2>Verified command</h2><pre>go run ./cmd/patchline db-semantics --engine postgres --sql demos/billing/migrations/002_bad_backfill.sql --out $LOCAL/db-semantics.json</pre><p>The command writes a JSON report with statement classification, lock notes, rollback feasibility, runtime-risk evidence, and conservative uncertainty when schema or table statistics are missing.</p></section><section class="card"><h2>Use it for</h2><p>Backfills, broad writes, DDL/data coupling, rollback review, lock-duration review, tenant-risk review, and migration-specific code review comments.</p></section></main></body></html>
EOF

cat > "$SITE/scenarios/ci-offline.html" <<EOF
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>CI and offline validation - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">Scenario</div><h1>Make analysis repeatable in CI and review.</h1>$scenario_nav</div></header><main><section class="card"><h2>GitHub Pages deployment</h2><p>The docs workflow checks out the repo, sets up Go, runs <code>bash scripts/build-docs-site.sh examples/docs-site-gate.json results/generated/docs-site</code>, uploads <code>results/generated/docs-site/site</code>, and deploys with <code>actions/deploy-pages</code>.</p></section><section class="card"><h2>Offline and redaction</h2><p>Normal cache-backed GitHub analyses can be checked later with offline validation. Redacted bundles are for sharing and intentionally replace sensitive paths with stable tokens, so validate offline before redaction.</p></section><section class="card"><h2>CI pattern</h2><pre>go test ./...
make docs-site-gate
make four-repo-demo</pre></section></main></body></html>
EOF

cat > "$SITE/reference/commands.html" <<'EOF'
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Command proof - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">Reference</div><h1>Every command on this page is generated from a successful local run.</h1><nav><a href="../index.html">Home</a><a href="commands.html">Commands</a><a href="artifacts.html">Artifacts</a><a href="validation.html">Validation</a></nav></div></header><main><section class="card"><h2>Machine-readable proof</h2><p>Open <a href="../artifacts/command-results.json">artifacts/command-results.json</a> for command strings, exit codes, stdout paths, and stderr paths.</p><table><tr><th>Command id</th><th>What it proves</th></tr><tr><td><code>about</code></td><td>The CLI explains the project purpose.</td></tr><tr><td><code>doctor</code></td><td>The demo workspace can be inspected.</td></tr><tr><td><code>intake</code></td><td>Repair evidence can be extracted from local files.</td></tr><tr><td><code>inventory</code></td><td>Files, facts, and project map artifacts are emitted.</td></tr><tr><td><code>baseline</code></td><td>Risks can be ranked from inventory plus intake.</td></tr><tr><td><code>propose</code></td><td>Quarantined deterministic review artifacts can be generated.</td></tr><tr><td><code>compare</code></td><td>Generated artifacts can be re-analyzed.</td></tr><tr><td><code>db-semantics</code></td><td>A SQL semantics report can be written.</td></tr><tr><td><code>plugins-list</code> / <code>plugins-probe</code></td><td>Extension points and project plugin coverage can be inspected.</td></tr></table></section></main></body></html>
EOF

cat > "$SITE/reference/artifacts.html" <<EOF
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Artifact reference - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">Reference</div><h1>Know which file to hand to which reviewer.</h1>$reference_nav</div></header><main><section class="card"><h2>Core artifacts</h2><table><tr><th>Artifact</th><th>Purpose</th></tr><tr><td><code>source.json</code></td><td>Repository/ref/subpath/cache metadata for fetched analyses.</td></tr><tr><td><code>facts.jsonl</code></td><td>Stable per-fact evidence extracted during inventory.</td></tr><tr><td><code>summary.sarif</code> / <code>baseline.sarif</code></td><td>CI and code-scanning friendly findings.</td></tr><tr><td><code>baseline.json</code></td><td>Ranked risks, evidence links, policy checks, recurrence, and proof holes.</td></tr><tr><td><code>proposal.patch</code></td><td>Generated review artifacts in patch form; not applied automatically.</td></tr><tr><td><code>compare.json</code></td><td>Deterministic re-analysis and generated-risk checks.</td></tr><tr><td><code>analysis-bundle/</code></td><td>Portable shareable bundle produced by <code>repo analyze</code>.</td></tr></table></section><section class="card"><h2>Site artifacts</h2><p>This docs site publishes <a href="../artifacts/public-demo.json">public-demo.json</a> and <a href="../artifacts/command-results.json">command-results.json</a> so the landing metrics and command claims are auditable.</p></section></main></body></html>
EOF

cat > "$SITE/reference/validation.html" <<EOF
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Validation reference - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">Reference</div><h1>Re-run the claims before you trust the site.</h1>$reference_nav</div></header><main><section class="card"><h2>Docs gate</h2><pre>make docs-site-gate</pre><p>The gate verifies required pages, role tutorials, GitHub Pages deployment config, public-demo thresholds, successful command samples, and internal links.</p></section><section class="card"><h2>Broader repository validation</h2><pre>go test ./...
PATCHLINE_PUBLIC_CORPUS_OFFLINE=1 make artifact-smoke
make artifact-benchmark-compare
make four-repo-demo</pre><p>Use the broader gates when documentation changes also update README claims, public-repo behavior, benchmark reports, or artifact expectations.</p></section></main></body></html>
EOF

touch "$SITE/.nojekyll"
cat > "$SITE/sitemap.xml" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>${site_url}</loc></url>
  <url><loc>${site_url}quickstart.html</loc></url>
  <url><loc>${site_url}architecture.html</loc></url>
  <url><loc>${site_url}scenarios/local-analysis.html</loc></url>
  <url><loc>${site_url}scenarios/public-repositories.html</loc></url>
  <url><loc>${site_url}scenarios/generated-interventions.html</loc></url>
  <url><loc>${site_url}scenarios/database-semantics.html</loc></url>
  <url><loc>${site_url}scenarios/ci-offline.html</loc></url>
  <url><loc>${site_url}tutorials/maintainers.html</loc></url>
  <url><loc>${site_url}tutorials/researchers.html</loc></url>
  <url><loc>${site_url}tutorials/security-reviewers.html</loc></url>
  <url><loc>${site_url}tutorials/contributors.html</loc></url>
  <url><loc>${site_url}reference/commands.html</loc></url>
  <url><loc>${site_url}reference/artifacts.html</loc></url>
  <url><loc>${site_url}reference/validation.html</loc></url>
</urlset>
EOF

jq -n \
  --arg site_url "$site_url" \
  --arg repo "$repo" \
  --arg ref "$ref" \
  --arg subpath "$subpath" \
  --argjson pages "$(find "$SITE" -type f | sed "s#^$SITE/##" | sort | jq -R . | jq -s .)" \
  --slurpfile demo "$SITE/artifacts/public-demo.json" \
  --slurpfile commands "$SITE/artifacts/command-results.json" \
  '{
    version:"patchline.docs-site/v1",
    site_url:$site_url,
    public_demo:{repo:$repo, ref:$ref, subpath:$subpath, summary:$demo[0]},
    command_results:$commands[0].summary,
    roles:["maintainers","researchers","security-reviewers","contributors"],
    pages:$pages
  }' > "$SITE/site-manifest.json"

echo "docs site built: $SITE pages $(jq '.pages | length' "$SITE/site-manifest.json") risks $risks"
