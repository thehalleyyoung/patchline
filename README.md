# Patchline

Patchline is a deterministic checker for data-change risk in code you already have: GitHub repos, local checkouts, migration folders, SQL files, ORM declarations, telemetry exports, incident notes, repair scripts, generated patches, and benchmark artifacts.

It is not an AI oracle and it does not require a Patchline-specific input format. Patchline inventories a project, extracts repo-native facts, ranks risky data changes, links evidence across files and time, creates bounded intervention artifacts when asked, and re-checks those artifacts before anyone treats them as useful.

## Choose your scenario

| Scenario | Use when | Start here |
| --- | --- | --- |
| Local first look | You have a repo, export, or migration folder and want a quick signal. | [Scan a local workspace](#scan-a-local-workspace) |
| Staged analysis | You want each artifact separately for review or CI. | [Run the staged pipeline](#run-the-staged-pipeline) |
| One-command bundle | You want fetch/inventory/intake/baseline/propose/compare output in one directory. | [Create a complete analysis bundle](#create-a-complete-analysis-bundle) |
| Public repo proof | You need to test Patchline on real ecosystems before trusting a README claim. | [Validate public repositories](#validate-public-repositories) |
| Generated intervention review | You want tests, guards, instrumentation, or repair candidates, but not auto-applied code. | [Review generated interventions safely](#review-generated-interventions-safely) |
| Database semantics | You want dialect-aware lock, rollback, query-plan, and runtime-risk notes for SQL. | [Check database semantics](#check-database-semantics) |
| Integrations | You want parser/plugin discovery, CI, SARIF, redaction, or offline replay. | [Integrate with existing workflows](#integrate-with-existing-workflows) |
| Artifact/research review | You need reproducible benchmark, phase, ground-truth, and paper-facing checks. | [Reproduce the artifact path](#reproduce-the-artifact-path) |

## Setup

From a checkout:

```bash
go build -o bin/patchline ./cmd/patchline
```

Check what the tool is for:

```bash
bin/patchline about
```

Most examples below write to `results/generated/...`. Those directories are generated outputs, not source-of-truth inputs.

## Scan a local workspace

Use this when you have a local repo, a migration directory, or an exported bundle. The committed `demos/billing` fixture is small enough for a quick smoke test but still contains a risky backfill migration, incident evidence, invariants, snapshots, and repair artifacts.

```bash
bin/patchline doctor demos/billing --out results/generated/readme/local-doctor
```

Doctor reports tool availability, scanned files, cache state, safe native checks, and next commands. Then run intake:

```bash
bin/patchline intake demos/billing --out results/generated/readme/local-intake
```

Intake writes `summary.json`, `summary.md`, and `summary.sarif`. It finds SQL files, loose SQL snippets, generic evidence signals, repair manifests, problem/cause/repair candidates, links, and time signals without labels.

## Run the staged pipeline

Use staged commands when you want each step checked into an artifact bundle or inspected independently.

```bash
bin/patchline repo inventory demos/billing --out results/generated/readme/inventory
```

Inventory writes `inventory.json`, `inventory.md`, `facts.jsonl`, and `project-map.md`. It detects languages, frameworks, migration systems, data-change files, test/native commands, CI, deploy config, telemetry-like exports, and low-level facts with stable provenance hashes.

```bash
bin/patchline repo baseline \
  --inventory results/generated/readme/inventory \
  --intake results/generated/readme/local-intake \
  --out results/generated/readme/baseline
```

Baseline writes `baseline.json`, `baseline.md`, and `baseline.sarif`. It ranks destructive SQL, broad writes, missing guards, weak rollback evidence, transaction/idempotency hazards, code-path write risk, cross-file evidence links, temporal windows, policy checks, recurrence signals, and proof holes.

```bash
bin/patchline repo playbook \
  --baseline results/generated/readme/baseline \
  --out results/generated/readme/playbook
```

Playbooks group hazards into owner-routed remediation steps with rollback points and handoff notes.

```bash
bin/patchline repo propose \
  --from-report results/generated/readme/baseline \
  --proposal-kind all \
  --budget files=4,lines=80,tokens=12000,changes=2 \
  --no-llm \
  --out results/generated/readme/proposal
```

Proposal writes `prompt-context.json`, `prompt.txt`, `proposal.patch`, `proposal.json`, `proposal.md`, and quarantined generated files. `--no-llm` means deterministic template-only generation. To use a generator, pass `--llm-command '<cmd>'`; Patchline sends the prompt on stdin and still treats the output as untrusted.

```bash
bin/patchline repo compare \
  --before results/generated/readme/baseline \
  --after results/generated/readme/proposal \
  --out results/generated/readme/compare
```

Compare writes `compare.json` and `compare.md`. It checks whether generated artifacts cover the targeted risk IDs, whether they add new risky SQL, whether Patchline checks pass, and whether native checks were unavailable, skipped, passed, or failed.

## Create a complete analysis bundle

Use `repo analyze` when you want the whole loop in one command:

```bash
bin/patchline repo analyze demos/billing \
  --stages inventory,baseline,propose,compare,deep \
  --proposal-kind all \
  --budget files=4,lines=80,tokens=12000,changes=2 \
  --no-llm \
  --out results/generated/readme/analysis
```

The output includes:

| Path | Purpose |
| --- | --- |
| `fetch/source.json` | Source metadata when the input is fetched from a remote repo. |
| `inventory/` | Inventory report, project map, and `facts.jsonl`. |
| `intake/` | Current-use problem/cause/repair candidate scan. |
| `baseline/` | Ranked risks, SARIF, evidence links, and proof holes. |
| `proposal/` | Quarantined intervention artifacts and prompt context. |
| `compare/` | Before/after deterministic checks. |
| `triage/` | Maintainer-oriented grouping by surface. |
| `analysis-bundle/` | Shareable summary, SARIF, facts, baseline, patch, compare, and commands. |
| `commands.md` | Copy/paste one-command and staged-command reproduction paths. |

Add `--resume` to reuse already-written fetch, inventory, intake, baseline, proposal, and compare artifacts. Add `--trace` to write JSONL diagnostics for long runs. Add `--ci` to emit CI metadata and SARIF upload hints.

## Validate public repositories

Patchline is meant to run on real repos that were not built for it. For this README rewrite, the command surface above was exercised against a public matrix covering the input classes called out in `100_STEPS.md`: Flask/Python with SQLite schema SQL, raw `.sql` migrator files, Django migrations, Airflow Alembic migrations, Rails Active Record migrations, Prisma, TypeORM, dbt models, Kubernetes data-service manifests, Protobuf schemas, and a MongoDB Prisma schema.

Representative public commands:

```bash
rm -rf results/generated/readme-public-matrix/cases/flask-tutorial
```

```bash
bin/patchline repo fetch pallets/flask \
  --subpath examples/tutorial \
  --out results/generated/readme-public-matrix/cases/flask-tutorial/fetch \
  --json
```

```bash
bin/patchline repo analyze --github pallets/flask \
  --subpath examples/tutorial \
  --stages inventory,baseline,propose,compare \
  --proposal-kind all \
  --budget files=3,lines=80,tokens=8000,changes=2 \
  --no-llm \
  --out results/generated/readme-public-matrix/cases/flask-tutorial/analyze \
  --json
```

```bash
bin/patchline repo offline \
  --analysis results/generated/readme-public-matrix/cases/flask-tutorial/analyze \
  --out results/generated/readme-public-matrix/cases/flask-tutorial/offline \
  --json
```

Validated public matrix:

| Slice | Ecosystem/evidence | Files | Facts | Risks | Generated | Offline |
| --- | --- | ---: | ---: | ---: | ---: | --- |
| `pallets/flask:examples/tutorial` | Python Flask, SQLite schema/source SQL | 22 | 62 | 28 | 3 | pass |
| `bytebase/bytebase:backend/migrator/migration` | Go SQL migrator, raw SQL migrations | 251 | 1144 | 1910 | 3 | pass |
| `django/django:django/contrib/auth/migrations` | Python Django migrations | 13 | 34 | 5 | 3 | pass |
| `apache/airflow:airflow-core/src/airflow/migrations/versions` | Python Airflow, Alembic migrations | 120 | 255 | 378 | 3 | pass |
| `lobsters/lobsters:db/migrate` | Ruby Rails, Active Record migrations | 158 | 442 | 328 | 3 | pass |
| `prisma/prisma-examples:orm/express/prisma` | TypeScript Prisma schema/migrations | 2 | 21 | 18 | 3 | pass |
| `typeorm/typeorm:test/functional/migrations` | TypeScript TypeORM migrations | 10 | 28 | 3 | 3 | pass |
| `dbt-labs/jaffle-shop:models` | SQL dbt models | 26 | 334 | 13 | 3 | pass |
| `kubernetes/examples:databases` | Kubernetes infra/data ordering manifests | 26 | 237 | 2 | 3 | pass |
| `protocolbuffers/protobuf:examples` | Protobuf schema files | 36 | 63 | 0 | 0 | pass |
| `prisma/prisma-examples:databases/mongodb/prisma` | MongoDB Prisma schema | 1 | 18 | 15 | 3 | pass |

For the built-in public corpus gate, run:

```bash
make four-repo-demo
```

That target fetches the configured public slice catalog, inventories each repo, builds facts, runs intake/baseline/propose/compare, verifies cache behavior, exercises `repo analyze --ci --redact --resume`, and writes a slice matrix under `results/generated/four-repo-analysis-demo/`.

## Review generated interventions safely

Patchline can create tests, guards, instrumentation, repair candidates, and explanation files, but generated code is always quarantined:

- It is written as artifacts under the proposal output, not silently applied to your active tree.
- It is bounded by `--budget files=N,lines=N,tokens=N,changes=N`.
- It carries prompt/context/output hashes.
- It is compared against the baseline before it is considered reviewable.
- Native project tests run only when explicitly requested with `--run-native-tests`; deterministic Patchline checks run regardless.

If you use a generator, keep the same compare step:

```bash
bin/patchline repo propose \
  --from-report results/generated/readme/baseline \
  --proposal-kind tests \
  --budget files=1,lines=120,tokens=4000,changes=3 \
  --llm-command "bash scripts/llm-command-smoke.sh" \
  --out results/generated/readme/llm-proposal
```

The smoke generator emits a small untrusted artifact with a prompt hash, proving generator plumbing and deterministic rejection/coverage behavior without relying on a hosted model or copying repository evidence into generated output.

## Check database semantics

Use `db-semantics` for dialect-aware SQL review:

```bash
bin/patchline db-semantics \
  --engine postgres \
  --sql demos/billing/migrations/002_bad_backfill.sql \
  --out results/generated/readme/db-semantics.json
```

The report classifies statements, lock mode/duration, reader/writer blocking, rollback feasibility, query-plan regression risk, runtime estimates when table hints are supplied, negative controls, and rule evidence. It is intentionally conservative when schema or runtime evidence is missing.

## Integrate with existing workflows

List the deterministic extension points:

```bash
bin/patchline plugins list
```

Probe a project for parser, fact extractor, linker, ranker, proposal, compare, and report-renderer coverage:

```bash
bin/patchline plugins probe demos/billing \
  --out results/generated/readme/plugins-probe
```

Use redaction for shareable bundles:

```bash
rm -rf results/generated/readme-public-matrix/redaction-smoke
```

```bash
bin/patchline repo analyze --github pallets/flask \
  --subpath examples/tutorial \
  --stages inventory,baseline,propose,compare \
  --proposal-kind all \
  --budget files=3,lines=80,tokens=8000,changes=2 \
  --redact \
  --no-llm \
  --out results/generated/readme-public-matrix/redaction-smoke \
  --json
```

Important limitation: `repo offline` validates cached source metadata. Run it on a normal cache-backed GitHub analysis. Do not run it on a redacted bundle, because redaction intentionally replaces cache paths with stable tokens.

Patchline also writes SARIF (`baseline.sarif`, `summary.sarif`) and CI helper files from `repo analyze --ci`; use those with GitHub Actions, GitLab Code Quality, or Bitbucket Code Insights.

## Reproduce the artifact path

Artifact-review commands use committed fixtures and frozen expectations. Start small:

```bash
bin/patchline semantics-audit --json
```

The default semantic audit uses bundled demo evidence and may report intentional counterexamples; that is a signal, not a crash.

```bash
bin/patchline artifact-ground-truth benchmarks --json
```

```bash
bin/patchline phase-check benchmarks/manifests/smoke.json --json
```

For the broader reviewer path:

```bash
make artifact-smoke
```

For full offline artifact validation:

```bash
PATCHLINE_PUBLIC_CORPUS_OFFLINE=1 make artifact-full
```

For the Go test suite:

```bash
go test ./...
```

## Generate a public-repo quickstart

`quickstart` does not download the repo. It writes a small plan with exactly three copy/paste commands and expected artifacts for the selected public slice.

```bash
bin/patchline quickstart \
  --github django/django \
  --subpath django/contrib/auth/migrations \
  --out results/generated/readme/quickstart
```

Use this when you want to hand a maintainer a minimal next-step plan instead of a full analysis bundle.

## What Patchline checks when evidence exists

| Evidence you have | Patchline can use it for |
| --- | --- |
| Migration files, raw SQL, ORM declarations | Schema evolution, destructive operations, broad writes, transaction/rollback/idempotency risk. |
| Source code and jobs | Persistent write breadth, affected tables, retry hazards, query shape, dataflow, native test commands. |
| Tests, fixtures, invariants | Guard suggestions, invariant mining, generated-test placement, proof-hole reduction. |
| Logs, traces, incidents, deploy markers | Temporal windows, trace-to-code links, recurrence, runtime confidence, causality-limit notes. |
| Repair scripts, runbooks, rollback notes | Problem/cause/repair clusters, remediation playbooks, scope/frame/rollback obligations. |
| CI and code-owner metadata | SARIF/code-quality output, owner routing, PR summaries, suppression expiry, what-changed reports. |
| Public benchmark manifests | Ground-truth checks, phase/input availability, ablations, result tables, reproducibility receipts. |

## Safety and limitations

Patchline favors explicit uncertainty over success-shaped defaults. If a repo lacks runtime evidence, production table sizes, before/after snapshots, or native test commands, reports say so instead of pretending the risk is proven. Generated interventions are review artifacts, not applied fixes. Redaction preserves joins and hashes but can make cache paths intentionally unreplayable for offline validation. Public-repo examples may move upstream; use pinned refs for papers, releases, or long-lived reproducibility claims.

## More documentation

- `ARTIFACT.md` — reviewer setup, artifact targets, expected outputs, and offline/public-corpus details.
- `100_STEPS.md` — the full roadmap and coverage checklist behind the scenario organization.
- `docs/architecture.md` — fetch, inventory, intake, baseline, proposal, compare, deep-analysis, and gate layers.
- `docs/plugin-interfaces.md` — parser/fact/link/rank/proposal/compare/report interfaces.
- `docs/threat-model.md` — untrusted repos, archives, generated code, native tests, and adapter inputs.
- `docs/generated-code-quarantine.md` — generated artifact handling and native-test safeguards.
