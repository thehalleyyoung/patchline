# Patchline

Patchline is a deterministic checker for the data-change material teams already have: GitHub repos, migration directories, service source trees, telemetry exports, JSON logs, incident notes, and repair scripts.

It is not an AI tool, and it does not require you to label data or adopt a Patchline-specific format first. Point it at existing files; it inventories what is there, finds risky SQL and operational clues, and prints the next commands that can run immediately.

```bash
go run ./cmd/patchline intake . --out results/generated/intake
go run ./cmd/patchline intake --github owner/repo --subpath path/to/migrations --out results/generated/intake
```

If you want a reproducible local workspace first, fetch and inventory the repo before intake:

```bash
go run ./cmd/patchline repo fetch django/django \
  --subpath django/contrib/auth/migrations \
  --out results/generated/repos/django-auth
go run ./cmd/patchline repo analyze --github django/django \
  --subpath django/contrib/auth/migrations \
  --stages inventory,baseline,propose,compare,deep \
  --budget files=15,lines=120,tokens=50000,changes=3 \
  --redact \
  --no-llm \
  --out results/generated/repos/django-auth-analysis
go run ./cmd/patchline repo inventory \
  results/generated/repos/django-auth/django-django-*/django/contrib/auth/migrations \
  --out results/generated/repos/django-auth-inventory

go run ./cmd/patchline intake \
  results/generated/repos/django-auth/django-django-*/django/contrib/auth/migrations \
  --out results/generated/repos/django-auth-intake

go run ./cmd/patchline repo baseline \
  --inventory results/generated/repos/django-auth-inventory \
  --intake results/generated/repos/django-auth-intake \
  --out results/generated/repos/django-auth-baseline

go run ./cmd/patchline repo propose \
  --from-report results/generated/repos/django-auth-baseline \
  --proposal-kind all \
  --budget files=15,lines=120,tokens=50000,changes=3 \
  --out results/generated/repos/django-auth-proposal

go run ./cmd/patchline repo compare \
  --before results/generated/repos/django-auth-baseline \
  --after results/generated/repos/django-auth-proposal \
  --out results/generated/repos/django-auth-compare
```

Fetch writes `source.json` with source provenance, resolved GitHub commit, archive hash, timestamp, tool version, and cache metadata. Inventory writes `inventory.json`, `inventory.md`, `facts.jsonl`, and `project-map.md`. Baseline writes `baseline.json`, `baseline.md`, and `baseline.sarif`. Propose writes `prompt-context.json`, `prompt.txt`, `proposal.patch`, `proposal.json`, and generated untrusted artifacts under `patchline-proposals/`. Compare writes `compare.json` and `compare.md`.

`repo analyze` also writes `commands.md`, a copy/paste maintainer report with the equivalent one-command and staged command sequences plus the shareable bundle paths.

To plug in a local or hosted generator, pass `--llm-command '<cmd>'`; Patchline sends the prompt on stdin and stores the output as an untrusted artifact for deterministic compare. Use `--no-llm` when you want template-only analysis and rejection of any generator command.

Use `--budget files=N,lines=N,tokens=N,changes=N` to bound generated scope before a patch is written. `changes` limits targeted risks, `files` limits generated artifacts, `lines` limits each artifact, and `tokens` limits approximate generated output tokens.

Rerun `repo analyze --resume --out <same-dir>` to reuse existing fetch, inventory, intake, baseline, proposal, and compare artifacts while changing later experiment settings.

Add `--redact` to write `analysis-bundle/` copies with stable redaction tokens for identifiers, literals, customer-like strings, and secret-like values while preserving joins and existing artifact hashes.

Add `--ci` to write `ci/summary.md` plus a GitHub Actions upload snippet for `github/codeql-action/upload-sarif` and `actions/upload-artifact`, pointing at `analysis-bundle/summary.sarif` and the full analysis bundle.

For example, with an OpenAI key already exported as `OPENAI_API_KEY` or `openai_api_key`, you can pass a command that reads Patchline's prompt from stdin and writes generated text to stdout:

```bash
go run ./cmd/patchline repo propose \
  --from-report results/generated/repos/django-auth-baseline \
  --proposal-kind tests \
  --llm-command 'python3 -c "import json, os, sys, urllib.request; prompt=sys.stdin.read(); key=os.environ.get(\"OPENAI_API_KEY\") or os.environ[\"openai_api_key\"]; body=json.dumps({\"model\":\"gpt-4o-mini\",\"messages\":[{\"role\":\"user\",\"content\":prompt}]}).encode(); req=urllib.request.Request(\"https://api.openai.com/v1/chat/completions\", data=body, headers={\"Authorization\":\"Bearer \"+key,\"Content-Type\":\"application/json\"}); print(json.load(urllib.request.urlopen(req))[\"choices\"][0][\"message\"][\"content\"])"' \
  --out results/generated/repos/django-auth-openai-proposal
```

## Why this is useful

Real production data problems often start as ordinary files: a broad migration, a risky backfill, a rollback note, a deploy export, a trace dump, or an ad hoc repair script. Those files are usually scattered across repos and tools. Patchline gives you a first pass over them without asking you to prepare a special dataset.

On the public Bytebase repository, this command downloads the repo and checks its real migration directory:

```bash
go run ./cmd/patchline intake \
  --github bytebase/bytebase \
  --subpath backend/migrator/migration \
  --out results/generated/intake
```

Recent output from that command:

```text
files=251 sql_files=251 high_risk=378 medium_risk=725
problems=339 causes=339 repair_candidates=16 links=10243
```

One concrete finding was a high-risk update in Bytebase's real migration history:

```text
3.1/0000##sheet_blob.sql
table=sheet
update sheet set sha256 = sha256(convert_to(sheet.statement, ?)) where statement is not null
```

Patchline also emits runnable follow-up commands such as:

```bash
patchline analyze-migration .../backend/migrator/migration/3.1/0000##sheet_blob.sql --json
```

That is the core value: before you write custom labels, manifests, or benchmark cases, Patchline can already tell you where risky data transitions, possible causes, and possible repair/rollback evidence are hiding in a real project.

## Plug-and-play demo on existing projects

Run one command to check several public projects and get a compact report:

```bash
make plug-and-play-demo
```

To run the staged fetch -> inventory -> intake -> baseline path on real GitHub repos:

```bash
make repo-demo
make four-repo-demo
make repo-slice-matrix
```

The demo downloads real GitHub project subpaths and writes:

```text
results/generated/plug-and-play-demo/summary.md
results/generated/plug-and-play-demo/summary.json
results/generated/plug-and-play-demo/cases/*/summary.sarif
results/generated/real-repo-slice-matrix/slice-matrix.md
results/generated/real-repo-slice-matrix/slice-matrix.json
```

The real-repo slice matrix is backed by `examples/real-repo-slices.json` and reports each public slice by ecosystem, migration framework, repo size class, available evidence types, fetched commit, inventory coverage, grep-only comparison, risks, linked candidates, time signals, generated artifacts, before/after deltas, and cache proof.

Current default projects:

| Project slice | Files | SQL files | Loose SQL | High-risk SQL | Problems | Causes | Repairs | Links | Time signals |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Bytebase migrations (`bytebase/bytebase:backend/migrator/migration`) | 251 | 251 | 0 | 378 | 339 | 339 | 16 | 10243 | 2 |
| Django auth migrations (`django/django:django/contrib/auth/migrations`) | 13 | 0 | 3 | 3 | 2 | 2 | 0 | 2 | 0 |
| Mastodon migrations (`mastodon/mastodon:db/migrate`) | 514 | 0 | 97 | 58 | 44 | 44 | 6 | 76 | 525 |

Each case directory contains the full intake report, SARIF for code-scanning systems, and command log. To scan your own project list:

```bash
PATCHLINE_PLUGPLAY_CASES=$'My app|owner/repo|path/to/migrations\nOther app|owner2/repo2|db/migrate' \
  bash scripts/plug-and-play-demo.sh results/generated/my-plugplay
```

## What intake finds

`patchline intake` produces `summary.json` and `summary.md` with:

| Finds | From |
| --- | --- |
| Source provenance | `source.json` from repo fetch: owner/repo, ref, resolved commit, subpath, archive hash, timestamp, tool version, and cache hit/path |
| One-command analysis | `analyze.json`, `analyze.md`, and `analysis-bundle/` from repo analyze: fetch, inventory, baseline, proposal, compare, and deep-analysis summaries in one staged run |
| Risky data-changing SQL | `.sql`, `.psql`, `.ddl`, and SQL snippets in text/source files |
| Project facts | `facts.jsonl` from repo inventory: files, languages, migrations, schema evolution, native commands, unknown JSON/YAML/TOML/log fields, docs, evidence exports, tables, columns, models, endpoints, queues, jobs, deploys, timestamps, errors, and next commands |
| Baseline ranking | `baseline.json`, `baseline.md`, and `baseline.sarif` from repo baseline: ranked SQL and code-path risks, ranking explanations with feature ablations, provenance slices, Datalog-style query rows, abstract effects, symbolic checks, temporal windows, recurrence patterns, policy checks, repair proof summaries, evidence links, native checks, and ablation counts |
| Bounded proposals | `proposal.patch` plus isolated untrusted tests, guards, instrumentation notes, repair manifests, explain SQL, and intervention metadata from repo propose |
| Proposal checks | `compare.json` and `compare.md` from repo compare: generated-artifact checks, coverage, intervention-loop status, risky generated SQL, optional safe native-test execution, and review status |
| Embedded/source SQL | Go, Python, Ruby, JavaScript/TypeScript, Java, C#, shell, and SQL files |
| Cause candidates | risky migrations, deploy/trace/commit/migration signals, incident-like notes, export fields |
| Repair candidates | repair manifests, rollback/revert/restore/backfill/reconcile/fix clues in SQL/docs/scripts |
| Time signals | dates in filenames and text that can slice a repo/export around likely migration, incident, deploy, or repair windows |
| Existing evidence | JSONL evidence, Datadog/OTLP/GitHub/Postgres/migration-runner exports when recognizable |
| Unknown JSON signals | generic SQL/trace/deploy/commit/record fields without requiring a known schema |
| SARIF output | `summary.sarif` for CI/code-scanning integration |
| Next commands | exact commands that should run on discovered files |

Candidate links are conservative: Patchline links problems, causes, and repairs only through shared identifiers such as table names, incident IDs, or commits. A link is a lead to inspect, not proof of causality.

## Common commands

```bash
# Build
go build -o bin/patchline ./cmd/patchline

# Scan current repo/export data
go run ./cmd/patchline intake . --out results/generated/intake

# Scan part of a public GitHub repo
go run ./cmd/patchline intake --github bytebase/bytebase --subpath backend/migrator/migration --out results/generated/intake

# Compare several existing public projects
make plug-and-play-demo

# Fetch, inventory, analyze, and rank one real GitHub repo slice
make repo-demo

# Prove the staged workflow across four real external repo slices
make four-repo-demo

# Use as a GitHub Action in another repo
# - uses: thehalleyyoung/patchline@main
#   with:
#     path: .
#     out: results/patchline-intake

# Analyze one migration directly
go run ./cmd/patchline analyze-migration path/to/migration.sql --json

# Extract embedded SQL from a source tree
go run ./cmd/patchline extract-sql path/to/source --json

# Adapt existing observability/deploy exports when they match known formats
go run ./cmd/patchline adapt-evidence datadog export.json --json
go run ./cmd/patchline adapt-evidence otlp export.json --json
go run ./cmd/patchline adapt-evidence postgres logical-decoding.json --json
go run ./cmd/patchline adapt-evidence github deployments.json --json
go run ./cmd/patchline adapt-evidence migration-runner migrations.json --json

# Lint or replay richer repair artifacts if you already have them
go run ./cmd/patchline lint-repair repair.json --json
go run ./cmd/patchline dry-run repair.json --store store.json --json
```

## What deeper checks add

The intake command is the front door. If your tree also contains richer inputs, Patchline can go further:

| If you have | Patchline can check |
| --- | --- |
| repair manifests | operation scope, dependency cycles, rollback requirements, generated SQL |
| bounded before/after stores | replayed row diffs and repair effects |
| invariants | row-count/frame/invariant obligations, with Z3 when available |
| event JSONL or adapted exports | provenance graphs, trace reconstruction, blast radius |
| prior incident archives | recurring risky tables, missing rollback patterns, repeated repair shapes |

Patchline always records when it cannot prove something. Missing evidence becomes an explicit downgrade, not a made-up success.

## Validation

Run the normal test suite:

```bash
go test ./...
```

Run the small intake demo bundled with this repo:

```bash
make intake-demo
```

Run the broader deterministic validation suite when you want to exercise the benchmark, provenance, baseline, ablation, and public-corpus checks:

```bash
PATCHLINE_PUBLIC_CORPUS_OFFLINE=1 make artifact-full
```

`artifact-full` is intentionally separate from live source checks and maintainer refresh targets. Expected-output refresh commands require explicit opt-in:

```bash
PATCHLINE_ACCEPT_EXPECTED_REFRESH=1 make artifact-benchmark-refresh
PATCHLINE_ACCEPT_EXPECTED_REFRESH=1 make artifact-studies-refresh
```

## Repository map

```text
cmd/patchline/          CLI entry point
internal/intake/       current-data scanner and GitHub project intake
internal/migration/    SQL and embedded-source-SQL analysis
internal/evidence/     JSONL ingest plus Datadog/OTLP/GitHub/Postgres adapters
internal/repair/       repair manifests, linting, SQL generation, dry-run support
internal/artifact/     deterministic validation reports and benchmark checks
examples/              small runnable examples and public-source-derived fixtures
benchmarks/            frozen validation inputs and expected outputs
docs/                  deeper design notes
```

## Current limits

Patchline is useful as a deterministic first pass and audit trail, not as an automatic incident resolver. It does not infer private production data, prove arbitrary SQL, synthesize repairs from prose, or replace operator review. It gives you grounded leads and runnable checks from the data already in front of you.
