#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/docs-site-gate.json}"
OUT="${2:-results/generated/docs-site}"
SITE="$OUT/site"
LOCAL="$OUT/local-checks"
rm -rf "$OUT"
mkdir -p "$SITE/tutorials" "$SITE/scenarios" "$SITE/reference" "$SITE/api" "$SITE/artifacts" "$OUT/cache" "$OUT/commands" "$LOCAL"

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

jq -n \
  --slurpfile slices examples/real-repo-slices.json \
  --slurpfile adjudications examples/real-repo-adjudications.json \
  --slurpfile publicIncidents benchmarks/expected/public-incidents-report.json \
  --slurpfile publicMigrations benchmarks/expected/public-migrations-report.json \
  --slurpfile publicArchive benchmarks/expected/public-archive-report.json \
  --slurpfile publicRepairs benchmarks/expected/public-repairs-report.json \
  --slurpfile smoke benchmarks/expected/smoke-report.json \
  '{
    version:"patchline.docs-public-evidence/v1",
    public_repo_slices:$slices[0].slices,
    slice_summary:{
      repos:($slices[0].slices | map(.repo) | unique | length),
      slices:($slices[0].slices | length),
      ecosystems:($slices[0].slices | map(.ecosystem) | unique)
    },
    sampled_adjudications:$adjudications[0].adjudications,
    benchmark_summary:{
      public_incident_cases:($publicIncidents[0].cases | length),
      public_migration_cases:($publicMigrations[0].cases | length),
      public_archive_cases:($publicArchive[0].cases | length),
      public_repair_cases:($publicRepairs[0].cases | length),
      smoke_cases:($smoke[0].cases | length),
      all_expected_ok:([
        $publicIncidents[0].cases[],
        $publicMigrations[0].cases[],
        $publicArchive[0].cases[],
        $publicRepairs[0].cases[],
        $smoke[0].cases[]
      ] | all(.ok == true))
    },
    confirmed_public_incident_ground_truth:[
      "benchmarks/ground_truth/public_incidents/gitlab-2017-public-source-observations.json",
      "benchmarks/ground_truth/public_incidents/github-2018-public-source-observations.json"
    ],
    note:"Public repo slices are review-risk evidence. Confirmed bug/incident claims are limited to labeled ground-truth benchmark cases and public postmortem-derived fixtures."
  }' > "$SITE/artifacts/public-evidence.json"

files="$(jq '.files_scanned' "$SITE/artifacts/public-demo.json")"
risks="$(jq '.ranked_risks' "$SITE/artifacts/public-demo.json")"
provenance="$(jq '.provenance_slices' "$SITE/artifacts/public-demo.json")"
generated="$(jq '.generated_files' "$SITE/artifacts/public-demo.json")"
failed="$(jq '.compare_checks_failed' "$SITE/artifacts/public-demo.json")"
slice_repos="$(jq '.slice_summary.repos' "$SITE/artifacts/public-evidence.json")"
slice_count="$(jq '.slice_summary.slices' "$SITE/artifacts/public-evidence.json")"
public_migration_cases="$(jq '.benchmark_summary.public_migration_cases' "$SITE/artifacts/public-evidence.json")"
public_incident_cases="$(jq '.benchmark_summary.public_incident_cases' "$SITE/artifacts/public-evidence.json")"
smoke_cases="$(jq '.benchmark_summary.smoke_cases' "$SITE/artifacts/public-evidence.json")"
confirmed_public_incidents="$(jq '.confirmed_public_incident_ground_truth | length' "$SITE/artifacts/public-evidence.json")"

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

root_nav='<nav><a href="index.html">Home</a><a href="quickstart.html">Quickstart</a><a href="architecture.html">Architecture</a><a href="theory.html">Theory</a><a href="api/index.html">Public repo API</a><a href="reference/findings.html">What it finds</a></nav>'
tutorial_nav='<nav><a href="../index.html">Home</a><a href="../quickstart.html">Quickstart</a><a href="maintainers.html">Maintainers</a><a href="researchers.html">Researchers</a><a href="security-reviewers.html">Security reviewers</a><a href="contributors.html">Contributors</a></nav>'
scenario_nav='<nav><a href="../index.html">Home</a><a href="../quickstart.html">Quickstart</a><a href="local-analysis.html">Local</a><a href="public-repositories.html">Public repos</a><a href="generated-interventions.html">Generated review</a><a href="database-semantics.html">DB semantics</a><a href="ci-offline.html">CI/offline</a></nav>'
reference_nav='<nav><a href="../index.html">Home</a><a href="../architecture.html">Architecture</a><a href="findings.html">Findings</a><a href="artifacts.html">Artifacts</a><a href="validation.html">Validation</a></nav>'
api_nav='<nav><a href="../index.html">Home</a><a href="index.html">API overview</a><a href="public-repo-lifecycle.html">Lifecycle</a><a href="fetch.html">Fetch</a><a href="analyze.html">Analyze</a><a href="staged-workflow.html">Staged</a><a href="interventions.html">Interventions</a><a href="offline-redaction-ci.html">Offline/CI</a><a href="outputs.html">Outputs</a></nav>'

cat > "$SITE/index.html" <<EOF
<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Patchline documentation</title><link rel="stylesheet" href="styles.css"></head>
<body>
<header><div class="hero"><div class="eyebrow">Verified GitHub Pages documentation</div><h1>Patchline finds data-change bugs before they become incidents.</h1><p>Deterministic analysis for maintainers, researchers, security reviewers, and contributors: fetch real repositories, inventory facts, rank risks, generate bounded review artifacts, and re-check every generated artifact before review.</p>$root_nav</div></header>
<main>
<section class="card"><h2>Real public-repo output</h2><p>This site build regenerated <code>$repo</code> <code>$subpath</code> at pinned commit <code>$ref</code>. The public evidence catalog also tracks <strong>$slice_count</strong> pinned public repo slices across <strong>$slice_repos</strong> repositories, plus public migration and incident benchmark cases.</p><div class="grid"><div><div class="metric">$files</div><p>public files scanned in the live demo</p></div><div><div class="metric">$risks</div><p>ranked risks in the live demo</p></div><div><div class="metric">$public_migration_cases</div><p>public migration benchmark cases</p></div><div><div class="metric">$confirmed_public_incidents</div><p>confirmed public incident bug cases</p></div><div><div class="metric">$failed</div><p>deterministic checks failed</p></div></div><p><a href="artifacts/public-demo.json">Download public-demo.json</a> and <a href="artifacts/public-evidence.json">public-evidence.json</a>.</p></section>
<section class="card"><h2>Empirically shown in this repo</h2><p>The repository evidence currently demonstrates four concrete things: Patchline ran end-to-end on a pinned public Lobsters migration slice; the public slice catalog covers Forem, Bytebase, Mastodon, and Lobsters; benchmark reports contain confirmed public incident cases for GitLab 2017 primary data loss and GitHub 2018 split-brain write divergence; and the analyzer emits specific review surfaces such as broad writes, transaction/idempotency gaps, lock hazards, privacy hazards, provenance slices, symbolic checks, policy failures, and generated-artifact compare checks.</p></section>
<section class="card"><h2>Confirmed bugs and real-repo risk findings</h2><p>Patchline has two evidence levels. Public repo sweeps find review risks across real migration ecosystems. Confirmed bug claims are limited to labeled ground-truth cases, including public GitLab 2017 primary data loss observations, GitHub 2018 split-brain write divergence observations, committed unsafe backfill fixtures, and pinned public Bytebase migration-risk labels. The site does not claim every public-repo warning is a confirmed production bug.</p></section>
<section class="card"><h2>Choose your path</h2><div class="grid"><div><h3>Run it</h3><p><a href="quickstart.html">Build and run the verified quickstart</a>, then inspect the artifact bundle.</p></div><div><h3>Understand it</h3><p><a href="architecture.html">Follow the fetch to compare pipeline</a> and the evidence each layer emits.</p></div><div><h3>Use it on public repos</h3><p><a href="api/index.html">Read the public-repo API manual</a>: fetch, analyze, staged workflows, intervention generation, offline validation, redaction, CI, and output files.</p></div><div><h3>Audit it</h3><p><a href="reference/validation.html">Re-run the gates</a> that back the site claims.</p></div></div></section>
<section class="card"><h2>Start by role</h2><ul><li><a href="tutorials/maintainers.html">Maintainers: first run, triage, generated review artifacts, and reviewer handoff.</a></li><li><a href="tutorials/researchers.html">Researchers: claims-to-evidence maps, public corpora, limitations, and reproducible artifacts.</a></li><li><a href="tutorials/security-reviewers.html">Security reviewers: threat model, generated-code quarantine, release checksums, and offline mode.</a></li><li><a href="tutorials/contributors.html">Contributors: local checks, fixtures, plugins, and focused gates.</a></li></ul></section>
</main><footer>Generated by <code>scripts/build-docs-site.sh</code>.</footer></body></html>
EOF

cat > "$SITE/quickstart.html" <<EOF
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Patchline quickstart</title><link rel="stylesheet" href="styles.css"></head><body><header><div class="hero"><div class="eyebrow">Quickstart</div><h1>From checkout to evidence bundle.</h1><p>Use this flow when you want a small local run before moving to public-repo or CI validation.</p>$root_nav</div></header><main><section class="card"><h2>Build and inspect</h2><pre>go build -o bin/patchline ./cmd/patchline
bin/patchline about
bin/patchline doctor demos/billing --out results/generated/docs-local/doctor</pre><p>The docs gate rebuilds this flow and fails if the command family stops producing the expected artifacts.</p></section><section class="card"><h2>Run the local pipeline</h2><pre>bin/patchline intake demos/billing --out results/generated/docs-local/intake
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

cat > "$SITE/theory.html" <<EOF
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Patchline risk theory</title><link rel="stylesheet" href="styles.css"></head><body><header><div class="hero"><div class="eyebrow">Theory of risk classification</div><h1>How Patchline decides what to call a risk.</h1><p>A computational account of the evidence graph, effect lattice, additive ranking model, proof obligations, and soundness boundary behind baseline risk reports.</p>$root_nav</div></header><main><section class="card"><h2>The decision problem</h2><p>Patchline calls something a risk when repository evidence shows a durable-state operation and local evidence has not established safe scope, rollback, idempotency, concurrency, privacy, and policy obligations. A risk is not automatically a confirmed bug; it is a reproducible request for review backed by paths, identifiers, factors, and proof holes.</p></section><section class="card"><h2>Computational model</h2><p>The baseline pipeline indexes extracted facts by canonical identifiers, ranks SQL statements and source/schema write observations, links risks to matching facts, builds provenance slices, assigns stable risk IDs, and then runs abstract effects, symbolic checks, transaction, idempotency, lock, privacy, policy, and repair-proof analyses.</p><pre>repository evidence
  -> facts, SQL statements, source observations
  -> finite effect lattice
  -> additive risk score
  -> evidence-linked review frontier
  -> symbolic checks, policies, and proof holes</pre></section><section class="card"><h2>Effect lattice</h2><table><tr><th>Effect</th><th>Meaning</th><th>Review rank</th></tr><tr><td><code>noop</code></td><td>No concrete row change.</td><td>0</td></tr><tr><td><code>idempotent_update</code></td><td>Bounded deterministic write whose repeated application is stable.</td><td>1</td></tr><tr><td><code>reversible_update</code></td><td>Bounded write with declared snapshot rollback.</td><td>2</td></tr><tr><td><code>replay</code></td><td>External replay operation with system-specific semantics.</td><td>3</td></tr><tr><td><code>derived_rebuild</code></td><td>Derived-state rebuild from source records.</td><td>4</td></tr><tr><td><code>append_only_external</code></td><td>Append-only external effect requiring a compensating action.</td><td>5</td></tr><tr><td><code>destructive</code></td><td>Delete or unbounded write that removes or may rewrite state.</td><td>6</td></tr><tr><td><code>unknown</code></td><td>Operation outside known transfer functions.</td><td>7</td></tr></table></section><section class="card"><h2>Ranking rule</h2><p>Risk score is the sum of named factors. Severity is <code>high</code> at score 90 or above, <code>medium</code> from 50 through 89, and <code>low</code> below 50. Examples include <code>high-risk-sql</code>, <code>medium-risk-sql</code>, <code>persistent-write-code-path</code>, <code>broad-write</code>, <code>missing-transaction-boundary</code>, <code>missing-idempotency</code>, and <code>weak-rollback-signal</code>.</p><p>In the local docs fixture, the baseline produced <strong>$local_risks</strong> ranked risks from <strong>$local_facts</strong> facts. In the live public-repo docs demo, Patchline ranked <strong>$risks</strong> risks in the pinned Lobsters migration slice.</p></section><section class="card"><h2>What follows from a risk</h2><p>Risk classification feeds follow-on checks: symbolic idempotency, reversibility, frame, and scope checks; transaction-boundary inference; idempotency classification; lock/concurrency hazards; privacy/retention hazards; policy obligations; repair proof summaries; and proof-hole minimization. Missing evidence becomes an explicit proof hole, not a fake success.</p></section><section class="card"><h2>Soundness boundary</h2><p>The useful guarantee is conditional: if extraction finds a persistent operation and it matches an implemented transfer or scoring rule, Patchline places it in the review frontier with an explanation at least as conservative as the matched rule. It does not guarantee every risky program operation is found, and it does not claim every risk is a confirmed bug.</p></section><section class="card"><h2>Full paper and reproducibility gate</h2><p>The full theory paper lives in <code>docs/theory-risk-paper.md</code>. Reproduce the implementation check with:</p><pre>make theory-risk-paper-gate</pre><p>That gate validates the paper's constants and terminology against the current implementation and a demo baseline run.</p></section></main></body></html>
EOF

cat > "$SITE/api/index.html" <<EOF
  <!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Public repo API manual - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">Public repo API manual</div><h1>Run Patchline on public repositories, inspect every stage, and know what happens next.</h1><p>Intended to work with any GitHub repo; tested on many.</p>$api_nav</div></header><main><section class="card"><h2>Empirical baseline for this manual</h2><p>The live docs build ran <code>repo analyze</code> on <code>$repo:$subpath</code> at <code>$ref</code>. It scanned <strong>$files</strong> files, ranked <strong>$risks</strong> risks, produced <strong>$generated</strong> generated review artifacts, and compare reported <strong>$failed</strong> failed deterministic checks. The public catalog also covers <strong>$slice_count</strong> pinned slices across <strong>$slice_repos</strong> public repos.</p></section><section class="card"><h2>API map</h2><table><tr><th>Need</th><th>API page</th><th>What happens</th></tr><tr><td>Download and pin a public source slice</td><td><a href="fetch.html">Fetch API</a></td><td>Writes source metadata, cache proof, archive hash, resolved commit, and scanned root.</td></tr><tr><td>Run the whole workflow in one command</td><td><a href="analyze.html">Analyze API</a></td><td>Fetches, inventories, intakes, ranks, proposes, compares, and writes an analysis bundle.</td></tr><tr><td>Inspect each stage separately</td><td><a href="staged-workflow.html">Staged workflow API</a></td><td>Runs inventory, baseline, playbook, propose, compare, and optional deep outputs as separate artifacts.</td></tr><tr><td>Generate review artifacts safely</td><td><a href="interventions.html">Intervention API</a></td><td>Writes quarantined tests/guards/explain/repair artifacts and re-checks them before review.</td></tr><tr><td>Share or replay evidence</td><td><a href="offline-redaction-ci.html">Offline/redaction/CI API</a></td><td>Uses cache-backed offline validation, redaction, SARIF, and CI helper output.</td></tr><tr><td>Know which files to inspect</td><td><a href="outputs.html">Output schema guide</a></td><td>Explains source, inventory, intake, baseline, proposal, compare, bundle, and site evidence JSON.</td></tr></table></section><section class="card"><h2>Evidence boundary</h2><p>Public-repo analysis finds review risks and evidence links; labeled benchmark reports are where this repo makes confirmed bug/incident claims. The manual keeps those separate so a reviewer can tell the difference between a real-repo warning and a confirmed ground-truth case.</p></section></main></body></html>
EOF

cat > "$SITE/api/public-repo-lifecycle.html" <<EOF
  <!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Public repo lifecycle - Patchline API</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">API lifecycle</div><h1>The public-repo lifecycle: source, facts, risks, interventions, replay.</h1>$api_nav</div></header><main><section class="card"><h2>Lifecycle overview</h2><ol><li><strong>Choose a repo, commit, and subpath.</strong> Use a full commit SHA when you want stable outputs. Branch names work for exploration but are not stable evidence.</li><li><strong>Fetch or analyze.</strong> <code>repo fetch</code> only materializes source metadata and cache data. <code>repo analyze --github</code> can fetch and run selected analysis stages in one command.</li><li><strong>Inventory facts.</strong> The inventory stage emits file counts, languages, migration roots, schema-evolution files, native commands, source SQL hints, infrastructure/config hints, and <code>facts.jsonl</code>.</li><li><strong>Extract intake evidence.</strong> Intake finds problem/cause/repair candidates, SQL findings, source SQL, candidate links, time signals, and SARIF when evidence exists.</li><li><strong>Rank baseline risks.</strong> Baseline combines inventory and intake into ranked risks, evidence links, provenance slices, policy checks, proof holes, symbolic checks, lock hazards, privacy hazards, and repair-proof summaries.</li><li><strong>Generate bounded review artifacts.</strong> Proposal emits quarantined tests, guards, explain files, repair manifests, or other review artifacts, bounded by the budget.</li><li><strong>Compare before trusting.</strong> Compare re-analyzes generated artifacts for coverage, new risky SQL, failed generated checks, risk-budget rejection, intervention-loop state, and optional native test results.</li><li><strong>Package, redact, replay, or upload.</strong> Analysis bundles can be shared, redacted, checked offline, attached to CI, or used to feed paper/reviewer reports.</li></ol></section><section class="card"><h2>What the docs build showed</h2><p>For the pinned Lobsters migration slice, the lifecycle completed with <strong>$files</strong> files scanned, <strong>$risks</strong> ranked risks, <strong>$provenance</strong> provenance slices, <strong>$generated</strong> generated review artifacts, and <strong>$failed</strong> compare failures.</p></section></main></body></html>
EOF

cat > "$SITE/api/fetch.html" <<EOF
  <!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Fetch API - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">API: repo fetch</div><h1>Fetch pins a public source slice before analysis.</h1>$api_nav</div></header><main><section class="card"><h2>Command shape</h2><pre>bin/patchline repo fetch owner/repo \\
    --ref &lt;commit-or-ref&gt; \\
    --subpath &lt;path-inside-repo&gt; \\
    --out results/generated/public/fetch \\
    --json</pre><p>Use <code>--ref</code> with a full commit SHA for durable evidence. Use <code>--subpath</code> to keep analysis scoped to the migration, schema, app, infra, or evidence directory being reviewed.</p></section><section class="card"><h2>Public example</h2><pre>bin/patchline repo fetch $repo \\
    --ref $ref \\
    --subpath $subpath \\
    --out results/generated/lobsters/fetch \\
    --json</pre></section><section class="card"><h2>What happens</h2><p>Patchline resolves the source, writes <code>source.json</code>, records owner/repo/ref/subpath, stores a cache path and archive hash when a GitHub archive is used, and exposes a local scanned root for downstream commands. The cache proof is what later makes offline validation meaningful.</p></section><section class="card"><h2>Review checklist</h2><ul><li>Confirm <code>resolved_commit</code> is a 40-character commit.</li><li>Confirm <code>archive_hash</code> starts with <code>sha256:</code>.</li><li>Confirm <code>scanned_root</code> points to the intended subpath.</li><li>Do not treat redacted cache paths as offline-validation inputs; validate first, redact second.</li></ul></section></main></body></html>
EOF

cat > "$SITE/api/analyze.html" <<EOF
  <!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Analyze API - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">API: repo analyze</div><h1>Analyze is the public-repo one-command bundle.</h1>$api_nav</div></header><main><section class="card"><h2>Command shape</h2><pre>bin/patchline repo analyze --github owner/repo \\
    --ref &lt;commit&gt; \\
    --subpath &lt;path&gt; \\
    --stages inventory,baseline,propose,compare,deep \\
    --proposal-kind all \\
    --budget files=6,lines=90,tokens=10000,changes=2 \\
    --no-llm \\
    --out results/generated/public/analysis \\
    --json</pre></section><section class="card"><h2>Public example used by this site</h2><pre>bin/patchline repo analyze --github $repo \\
    --ref $ref \\
    --subpath $subpath \\
    --stages inventory,baseline,propose,compare \\
    --proposal-kind all \\
    --budget files=6,lines=90,tokens=10000,changes=2 \\
    --no-llm \\
    --out results/generated/docs-site/analysis \\
    --json</pre></section><section class="card"><h2>What happens</h2><p>Analyze writes stage directories plus an <code>analysis-bundle/</code>. In the site build it scanned <strong>$files</strong> public files, ranked <strong>$risks</strong> risks, wrote <strong>$generated</strong> generated review artifacts, and compare found <strong>$failed</strong> deterministic failures. With <code>--ci</code>, analyze also emits CI helper metadata and upload hints. With <code>--redact</code>, shareable bundles replace sensitive local details with stable tokens.</p></section><section class="card"><h2>Stage selection</h2><table><tr><th>Stage list</th><th>Use it when</th></tr><tr><td><code>inventory</code></td><td>You only need files, languages, migration roots, commands, and facts.</td></tr><tr><td><code>inventory,baseline</code></td><td>You want ranked risks but no generated artifacts.</td></tr><tr><td><code>inventory,baseline,propose,compare</code></td><td>You want the normal review loop.</td></tr><tr><td><code>inventory,baseline,propose,compare,deep</code></td><td>You want proof holes, recurrence, policies, and richer reviewer evidence.</td></tr></table></section></main></body></html>
EOF

cat > "$SITE/api/staged-workflow.html" <<EOF
  <!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Staged public repo workflow - Patchline API</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">API: staged workflow</div><h1>Use staged commands when each artifact needs review.</h1>$api_nav</div></header><main><section class="card"><h2>Public staged workflow</h2><pre>bin/patchline repo fetch $repo --ref $ref --subpath $subpath --out results/generated/lobsters/fetch --json
  bin/patchline repo inventory results/generated/lobsters/fetch/&lt;scanned-root&gt; --out results/generated/lobsters/inventory --json
  bin/patchline intake --github $repo --ref $ref --subpath $subpath --out results/generated/lobsters/intake
  bin/patchline repo baseline --inventory results/generated/lobsters/inventory --intake results/generated/lobsters/intake --out results/generated/lobsters/baseline --json
  bin/patchline repo playbook --baseline results/generated/lobsters/baseline --out results/generated/lobsters/playbook
  bin/patchline repo propose --from-report results/generated/lobsters/baseline --proposal-kind all --budget files=6,lines=90,tokens=10000,changes=2 --no-llm --out results/generated/lobsters/proposal --json
  bin/patchline repo compare --before results/generated/lobsters/baseline --after results/generated/lobsters/proposal --out results/generated/lobsters/compare --json</pre><p>The staged form makes it easier to archive or review each intermediate file. The one-command analyze API writes the same conceptual chain without requiring you to name every directory.</p></section><section class="card"><h2>What each stage gives you</h2><table><tr><th>Stage</th><th>Primary outputs</th><th>Reviewer question</th></tr><tr><td>fetch</td><td><code>source.json</code>, cache/archive metadata</td><td>Did we analyze the intended commit and subpath?</td></tr><tr><td>inventory</td><td><code>inventory.json</code>, <code>inventory.md</code>, <code>facts.jsonl</code>, <code>project-map.md</code></td><td>What files, commands, and facts exist?</td></tr><tr><td>intake</td><td><code>summary.json</code>, <code>summary.md</code>, <code>summary.sarif</code></td><td>What problem/cause/repair evidence is present?</td></tr><tr><td>baseline</td><td><code>baseline.json</code>, <code>baseline.md</code>, <code>baseline.sarif</code></td><td>Which risks should be reviewed first?</td></tr><tr><td>playbook</td><td>Maintainer handoff docs</td><td>What should an owner do next?</td></tr><tr><td>propose</td><td><code>proposal.json</code>, <code>proposal.md</code>, <code>proposal.patch</code>, generated files</td><td>What bounded review artifacts were created?</td></tr><tr><td>compare</td><td><code>compare.json</code>, <code>compare.md</code></td><td>Did generated artifacts improve coverage without adding risk?</td></tr></table></section></main></body></html>
EOF

cat > "$SITE/api/interventions.html" <<EOF
  <!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Intervention API - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">API: propose and compare</div><h1>Generated interventions are quarantined until compare passes.</h1>$api_nav</div></header><main><section class="card"><h2>Generate deterministic review artifacts</h2><pre>bin/patchline repo propose \\
    --from-report results/generated/lobsters/baseline \\
    --proposal-kind all \\
    --budget files=6,lines=90,tokens=10000,changes=2 \\
    --no-llm \\
    --out results/generated/lobsters/proposal \\
    --json</pre><p><code>--proposal-kind</code> controls the artifact family: tests, guards, explain notes, instrumentation, repair manifests, or <code>all</code>. <code>--budget</code> caps files, lines, token budget, and target changes. <code>--no-llm</code> uses deterministic templates; <code>--llm-command</code> sends the prompt to an explicit local command and still treats output as untrusted.</p></section><section class="card"><h2>Compare before review</h2><pre>bin/patchline repo compare \\
    --before results/generated/lobsters/baseline \\
    --after results/generated/lobsters/proposal \\
    --out results/generated/lobsters/compare \\
    --json</pre><p>Compare checks targeted-risk coverage, generated-file checks, new high/medium-risk SQL, risk budget, transaction and idempotency classifications, lock hazards, privacy hazards, and optional native test results.</p></section><section class="card"><h2>Empirical result in this site</h2><p>The docs public demo generated <strong>$generated</strong> review artifacts and compare reported <strong>$failed</strong> deterministic failures. The local small fixture generated <strong>$local_generated</strong> artifacts with <strong>$local_failed</strong> failed Patchline checks. These are examples of compare outcomes, not blanket guarantees for every repo.</p></section></main></body></html>
EOF

cat > "$SITE/api/offline-redaction-ci.html" <<EOF
  <!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Offline, redaction, and CI API - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">API: offline, redaction, CI</div><h1>Replay evidence, share safely, and wire CI without changing the source repo.</h1>$api_nav</div></header><main><section class="card"><h2>Offline validation</h2><pre>bin/patchline repo offline \\
    --analysis results/generated/public/analysis \\
    --out results/generated/public/offline \\
    --json</pre><p>Offline validation is for normal cache-backed GitHub analyses. It checks that cached source metadata and archive hashes still match. Validate offline before redaction; redaction intentionally replaces cache paths with stable tokens.</p></section><section class="card"><h2>Redacted public analysis</h2><pre>bin/patchline repo analyze --github $repo \\
    --ref $ref \\
    --subpath $subpath \\
    --stages inventory,baseline,propose,compare \\
    --proposal-kind all \\
    --redact \\
    --no-llm \\
    --out results/generated/lobsters-redacted \\
    --json</pre><p>Use redaction when a bundle needs to leave the trusted workspace. The bundle remains useful for review, but should not be used as the source for cache-path validation.</p></section><section class="card"><h2>CI mode</h2><pre>bin/patchline repo analyze --github $repo \\
    --ref $ref \\
    --subpath $subpath \\
    --stages inventory,baseline,propose,compare,deep \\
    --proposal-kind all \\
    --ci \\
    --no-llm \\
    --out results/generated/lobsters-ci \\
    --json</pre><p>CI mode emits SARIF and upload hints so findings can be attached to code-scanning or workflow artifacts. Native project tests remain opt-in and should run only in controlled CI or isolated worktrees.</p></section></main></body></html>
EOF

cat > "$SITE/api/outputs.html" <<EOF
  <!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Output schemas - Patchline API</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">API: outputs</div><h1>What files appear, and what they mean.</h1>$api_nav</div></header><main><section class="card"><h2>Analysis bundle files</h2><table><tr><th>File or directory</th><th>Meaning</th></tr><tr><td><code>fetch/source.json</code></td><td>Mode, input, owner, repo, ref, resolved commit, subpath, cache key, archive hash, cache path, scanned root.</td></tr><tr><td><code>inventory/inventory.json</code></td><td>Files scanned, languages, migration roots, schema evolution, source SQL hints, native commands, infrastructure and evidence categories.</td></tr><tr><td><code>inventory/facts.jsonl</code></td><td>One fact per line with stable IDs, kinds, paths, identifiers, confidence, and provenance properties.</td></tr><tr><td><code>intake/summary.json</code></td><td>Problem/cause/repair candidates, SQL findings, source SQL, candidate links, suggestions, source metadata, and hash.</td></tr><tr><td><code>baseline/baseline.json</code></td><td>Ranked risks, ranking explanations, evidence links, cause clusters, provenance slices, Datalog rows, abstract effects, symbolic checks, temporal windows, transaction/idempotency/lock/privacy hazards, invariant candidates, policy checks, repair proofs, and proof holes.</td></tr><tr><td><code>proposal/proposal.json</code></td><td>Proposal kind, generator, deterministic flag, prompt mode, budget, target risk IDs, context hash, output hash, intervention metadata, quarantine rules, generated file list.</td></tr><tr><td><code>proposal/proposal.patch</code></td><td>Generated review files represented as a patch, not applied to the public repo checkout.</td></tr><tr><td><code>compare/compare.json</code></td><td>Risk deltas, generated checks, transaction/idempotency/lock/privacy re-analysis, intervention loop, review badge, quarantine status, summary counts.</td></tr><tr><td><code>analysis-bundle/</code></td><td>Portable bundle with summary, SARIF, facts, baseline, proposal patch, compare report, and reproduction commands.</td></tr></table></section><section class="card"><h2>Docs evidence files</h2><p><a href="../artifacts/public-demo.json">public-demo.json</a> is the docs build's live public-repo result. <a href="../artifacts/public-evidence.json">public-evidence.json</a> lists the public slice catalog, sampled adjudications, public benchmark case counts, and confirmed incident ground-truth paths.</p></section></main></body></html>
EOF

cat > "$SITE/tutorials/maintainers.html" <<EOF
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Maintainer tutorial - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">Maintainers</div><h1>Turn a risky migration review into a bounded evidence packet.</h1>$tutorial_nav</div></header><main><section class="card"><h2>First-run analysis</h2><pre>go run ./cmd/patchline repo analyze --github $repo --ref $ref --subpath $subpath --stages inventory,baseline,propose,compare --no-llm --out results/patchline-demo</pre><p>This site build ran that public shape and observed <strong>$risks</strong> ranked public risks with <strong>$generated</strong> generated review artifacts. Review <code>baseline/baseline.md</code>, <code>proposal/proposal.md</code>, and <code>compare/compare.md</code>.</p></section><section class="card"><h2>Daily workflow</h2><ol><li>Run local inventory/intake on the changed slice.</li><li>Read the top ranked risks and linked evidence.</li><li>Generate bounded tests, guards, or instrumentation with <code>--no-llm</code> or a local <code>--llm-command</code>.</li><li>Trust only artifacts that pass compare and human review.</li></ol></section><section class="card"><h2>Review handoff</h2><p>Attach the analysis bundle, SARIF, and generated patch as review evidence. Use <a href="../reference/artifacts.html">the artifact reference</a> to explain what each file means.</p></section></main></body></html>
EOF

cat > "$SITE/tutorials/researchers.html" <<'EOF'
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Researcher tutorial - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">Researchers</div><h1>Write claims that point to artifacts.</h1><nav><a href="../index.html">Home</a><a href="../reference/validation.html">Validation</a><a href="maintainers.html">Maintainers</a><a href="researchers.html">Researchers</a><a href="security-reviewers.html">Security reviewers</a><a href="contributors.html">Contributors</a></nav></div></header><main><section class="card"><h2>Claims-to-evidence discipline</h2><p>Map abstract, introduction, and evaluation claims to concrete artifacts, limitations, missing evidence, paper wording, reviewer checks, and affected repositories.</p></section><section class="card"><h2>Reproducible public slices</h2><p>Public examples should be pinned by repository, commit, and subpath. This site uses the same pattern for its embedded public demo and exposes the generated JSON for inspection.</p></section><section class="card"><h2>Artifact path</h2><p>The validation reference lists the exact gates used to keep benchmark, public-repo, and documentation claims synchronized.</p></section></main></body></html>
EOF

cat > "$SITE/tutorials/security-reviewers.html" <<'EOF'
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Security reviewer tutorial - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">Security reviewers</div><h1>Use Patchline as an evidence gate, not a patch approval robot.</h1><nav><a href="../index.html">Home</a><a href="../scenarios/generated-interventions.html">Quarantine</a><a href="../scenarios/ci-offline.html">Offline</a><a href="maintainers.html">Maintainers</a><a href="researchers.html">Researchers</a><a href="security-reviewers.html">Security reviewers</a><a href="contributors.html">Contributors</a></nav></div></header><main><section class="card"><h2>Security review workflow</h2><ol><li><strong>Establish source identity.</strong> Check <code>source.json</code> first: repository, commit, subpath, archive hash, cache path, and scanned root must match the change under review.</li><li><strong>Review the baseline before the generated output.</strong> Open <code>baseline.md</code> and <code>baseline.json</code>; focus on high-score broad writes, destructive operations, weak rollback evidence, missing idempotency, privacy/retention hazards, and proof holes.</li><li><strong>Treat generated artifacts as hostile input.</strong> Generated files under <code>proposal/</code> are quarantined review material. They are not applied to the source tree and should not be copied into production until compare and human review agree.</li><li><strong>Use compare as the approval boundary.</strong> Open <code>compare.json</code>; block approval if generated artifacts add high-risk SQL, fail Patchline checks, exceed risk budget, or only cover risks by adding vague documentation.</li><li><strong>Escalate native execution deliberately.</strong> Project-native tests are skipped by default. Only use <code>--run-native-tests</code> in an isolated worktree or CI job where dependencies, network access, and secrets are controlled.</li><li><strong>Redact after verification.</strong> Run offline/cache validation on the original bundle first, then redact before sharing outside the trusted review boundary.</li></ol></section><section class="card"><h2>What to approve, reject, or ask for</h2><table><tr><th>Signal</th><th>Security reviewer action</th></tr><tr><td><code>source.json</code> has a pinned commit and archive hash</td><td>Proceed to risk review; the analysis input is reproducible.</td></tr><tr><td>Baseline shows broad writes without guards or rollback evidence</td><td>Ask for a bounded guard, dry-run evidence, rollback note, or table/tenant scoping before merge.</td></tr><tr><td>Compare reports failed Patchline checks or new risky SQL</td><td>Reject the generated intervention; keep it quarantined and request a narrower proposal.</td></tr><tr><td>Native checks are skipped</td><td>Do not treat that as failure by itself; decide whether an isolated native-test run is required for this change.</td></tr><tr><td>Bundle needs to leave the team</td><td>Require redaction and share the redacted bundle plus hashes, not raw local paths or cache locations.</td></tr></table></section><section class="card"><h2>Threat model</h2><p>Patchline treats fetched repositories, generated interventions, evidence adapters, and native project commands as untrusted inputs. The useful security property is the separation between <em>analysis evidence</em>, <em>quarantined generated artifacts</em>, and <em>human approval</em>.</p></section><section class="card"><h2>Controls to verify</h2><ul><li>Content-addressed archive metadata in <code>source.json</code>.</li><li>Redaction before sharing bundles outside a trusted boundary.</li><li>Native tests skipped unless explicitly requested with <code>--run-native-tests</code>.</li><li>Compare reports for new risky SQL and failed Patchline checks.</li><li>Public-demo proof in <a href="../artifacts/public-demo.json">public-demo.json</a> and public benchmark evidence in <a href="../artifacts/public-evidence.json">public-evidence.json</a>.</li></ul></section><section class="card"><h2>Release integrity</h2><p>Use signed release checksums, supply-chain provenance gates, and offline validation before trusting release or CI artifacts.</p></section></main></body></html>
EOF

cat > "$SITE/tutorials/contributors.html" <<'EOF'
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Contributor tutorial - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">Contributors</div><h1>Add capabilities without breaking evidence.</h1><nav><a href="../index.html">Home</a><a href="../reference/findings.html">Findings</a><a href="../reference/validation.html">Validation</a><a href="maintainers.html">Maintainers</a><a href="researchers.html">Researchers</a><a href="security-reviewers.html">Security reviewers</a><a href="contributors.html">Contributors</a></nav></div></header><main><section class="card"><h2>Local checks</h2><p>Run the repository tests and the focused gate for your feature before opening a PR. Documentation changes should still pass the docs-site gate.</p></section><section class="card"><h2>Fixtures and plugins</h2><p>Use the verified plugin commands, parser/fact/ranker interfaces, golden fixtures, fuzz seeds, and compatibility gates to add ecosystems safely.</p></section><section class="card"><h2>Documentation standard</h2><p>If you document a command, add it to the command-results artifact or to a focused make gate so future docs builds can prove it still runs.</p></section></main></body></html>
EOF

cat > "$SITE/scenarios/local-analysis.html" <<EOF
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Local analysis scenario - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">Scenario</div><h1>Scan the repo or migration folder you already have.</h1>$scenario_nav</div></header><main><section class="card"><h2>Verified local flow</h2><pre>go run ./cmd/patchline doctor demos/billing --out $LOCAL/doctor
go run ./cmd/patchline intake demos/billing --out $LOCAL/intake
go run ./cmd/patchline repo inventory demos/billing --out $LOCAL/inventory --json
go run ./cmd/patchline repo baseline --inventory $LOCAL/inventory --intake $LOCAL/intake --out $LOCAL/baseline --json</pre><p>The generated proof for those commands is in <a href="../artifacts/command-results.json">command-results.json</a>. The current local fixture produced <strong>$local_risks</strong> ranked risks from <strong>$local_facts</strong> facts.</p></section><section class="card"><h2>When to use it</h2><p>Use local analysis for migration folders, exported evidence bundles, or a checked-out service before you spend time wiring CI.</p></section></main></body></html>
EOF

cat > "$SITE/scenarios/public-repositories.html" <<EOF
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Public repository scenario - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">Scenario</div><h1>It runs on real repos, not Patchline-shaped fixtures.</h1>$scenario_nav</div></header><main><section class="card"><h2>Pinned public demo</h2><pre>go run ./cmd/patchline repo analyze --github $repo --ref $ref --subpath $subpath --stages inventory,baseline,propose,compare --proposal-kind all --budget files=6,lines=90,tokens=10000,changes=2 --no-llm --out results/generated/docs-site/analysis --json</pre><p>This exact site build regenerated the public demo with <strong>$files</strong> files, <strong>$risks</strong> ranked risks, and <strong>$generated</strong> generated review artifacts.</p></section><section class="card"><h2>Many-repo evidence</h2><p>The public evidence catalog covers <strong>$slice_count</strong> pinned real-repo slices across <strong>$slice_repos</strong> repositories: Forem, Bytebase, Mastodon, and Lobsters. The benchmark layer adds <strong>$public_migration_cases</strong> pinned public Bytebase migration cases, <strong>$public_incident_cases</strong> public incident cases, public-derived repair/archive boundaries, and <strong>$smoke_cases</strong> smoke cases.</p><p><a href="../artifacts/public-evidence.json">public-evidence.json</a> records the slice catalog, sampled adjudications, and benchmark case counts.</p></section><section class="card"><h2>Confirmed bug boundary</h2><p>Patchline does not claim every public-repo risk is a confirmed bug. Confirmed bug/incident claims are reserved for labeled ground-truth cases, including GitLab 2017 public primary-data-loss observations, GitHub 2018 public split-brain write-divergence observations, committed unsafe backfill fixtures, and pinned public migration-risk labels.</p></section></main></body></html>
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

cat > "$SITE/reference/findings.html" <<EOF
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>What Patchline finds</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">Reference</div><h1>What Patchline can find, with evidence boundaries.</h1>$reference_nav</div></header><main><section class="card"><h2>Repository and project structure</h2><p>Patchline inventories languages, file counts, byte counts, migration roots, migration systems, schema-evolution files, source SQL hints, NoSQL change hints, framework signals, native commands, CI files, deploy configuration, infrastructure files, operational docs, evidence exports, and field evidence. In the docs fixture it found <strong>$local_files</strong> files, <strong>$local_facts</strong> facts, <strong>$local_risks</strong> ranked risks, and source SQL hints in a small billing migration slice.</p></section><section class="card"><h2>Risky data changes</h2><p>Patchline ranks high-risk SQL, destructive operations, broad writes, code-path write risks, weak rollback evidence, missing guards, missing transaction boundaries, idempotency gaps, lock/concurrency hazards, privacy/retention hazards, tenant/table blast radius, and policy failures. It records why a risk was ranked through feature contributions and stable evidence hashes.</p></section><section class="card"><h2>Cross-file and historical evidence</h2><p>When evidence exists, Patchline links identifiers across files, clusters problem/cause/repair candidates, extracts provenance slices, tracks temporal windows, detects recurrence patterns, mines invariant candidates, estimates blast radius, and records proof holes instead of silently pretending missing evidence is safe.</p></section><section class="card"><h2>Program-analysis artifacts</h2><p>Baseline reports can include Datalog-style rows, abstract effects, symbolic checks, transaction-boundary inference, idempotency classifications, lock hazard classifications, privacy hazard classifications, proof-hole minimization, policy checks, repair-proof summaries, owner routes, and grep/sql/identifier-only comparator counts.</p></section><section class="card"><h2>Generated-intervention checks</h2><p>Compare reports can find whether generated artifacts cover targeted risks, add new high- or medium-risk SQL, fail generated-artifact checks, exceed risk budgets, lack fail-closed guards, include risky repair manifests, or need isolated native test execution. In the docs fixture compare accepted <strong>$local_generated</strong> generated review artifacts with <strong>$local_failed</strong> failed Patchline checks.</p></section><section class="card"><h2>Confirmed bug and incident classes</h2><p>Confirmed claims come from labeled ground truth: public GitLab 2017 primary data loss, public GitHub 2018 split-brain write divergence, committed unsafe broad-backfill fixtures, public Bytebase migration-risk labels, repair replay cases, semantic regression archives, and public-derived repair/archive boundaries. Public repo sweeps are review-risk evidence; confirmed bug language is reserved for those labeled benchmark cases.</p></section></main></body></html>
EOF

cat > "$SITE/reference/artifacts.html" <<EOF
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Artifact reference - Patchline</title><link rel="stylesheet" href="../styles.css"></head><body><header><div class="hero"><div class="eyebrow">Reference</div><h1>Know which file to hand to which reviewer.</h1>$reference_nav</div></header><main><section class="card"><h2>Core artifacts</h2><table><tr><th>Artifact</th><th>Purpose</th></tr><tr><td><code>source.json</code></td><td>Repository/ref/subpath/cache metadata for fetched analyses.</td></tr><tr><td><code>facts.jsonl</code></td><td>Stable per-fact evidence extracted during inventory.</td></tr><tr><td><code>summary.sarif</code> / <code>baseline.sarif</code></td><td>CI and code-scanning friendly findings.</td></tr><tr><td><code>baseline.json</code></td><td>Ranked risks, evidence links, policy checks, recurrence, and proof holes.</td></tr><tr><td><code>proposal.patch</code></td><td>Generated review artifacts in patch form; not applied automatically.</td></tr><tr><td><code>compare.json</code></td><td>Deterministic re-analysis and generated-risk checks.</td></tr><tr><td><code>analysis-bundle/</code></td><td>Portable shareable bundle produced by <code>repo analyze</code>.</td></tr></table></section><section class="card"><h2>Site artifacts</h2><p>This docs site publishes <a href="../artifacts/public-demo.json">public-demo.json</a> and <a href="../artifacts/public-evidence.json">public-evidence.json</a> so the public-repo and confirmed-benchmark claims are auditable.</p></section></main></body></html>
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
  <url><loc>${site_url}theory.html</loc></url>
  <url><loc>${site_url}api/index.html</loc></url>
  <url><loc>${site_url}api/public-repo-lifecycle.html</loc></url>
  <url><loc>${site_url}api/fetch.html</loc></url>
  <url><loc>${site_url}api/analyze.html</loc></url>
  <url><loc>${site_url}api/staged-workflow.html</loc></url>
  <url><loc>${site_url}api/interventions.html</loc></url>
  <url><loc>${site_url}api/offline-redaction-ci.html</loc></url>
  <url><loc>${site_url}api/outputs.html</loc></url>
  <url><loc>${site_url}scenarios/local-analysis.html</loc></url>
  <url><loc>${site_url}scenarios/public-repositories.html</loc></url>
  <url><loc>${site_url}scenarios/generated-interventions.html</loc></url>
  <url><loc>${site_url}scenarios/database-semantics.html</loc></url>
  <url><loc>${site_url}scenarios/ci-offline.html</loc></url>
  <url><loc>${site_url}tutorials/maintainers.html</loc></url>
  <url><loc>${site_url}tutorials/researchers.html</loc></url>
  <url><loc>${site_url}tutorials/security-reviewers.html</loc></url>
  <url><loc>${site_url}tutorials/contributors.html</loc></url>
  <url><loc>${site_url}reference/findings.html</loc></url>
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
