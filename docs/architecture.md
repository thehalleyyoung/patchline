# Patchline architecture

Patchline is a deterministic data and code repair workbench. It turns an existing project slice into a reproducible chain of facts, risks, proposed interventions, re-analysis results, and CI gates. The design goal is simple: every claim in a report should point to local evidence, every generated repair should be treated as untrusted until it is analyzed again, and every enterprise workflow should work with cached inputs and without production credentials.

## Layer map

```text
project source or evidence export
  |
  v
fetch -> inventory -> intake -> baseline -> proposal -> compare -> deep-analysis -> gate
  |        |           |         |          |           |           |              |
  |        |           |         |          |           |           |              +-- pass/fail policy for CI and review
  |        |           |         |          |           |           +---------------- proof holes, symbolic checks, recurrence, policy status
  |        |           |         |          |           +---------------------------- deterministic re-analysis of generated intervention files
  |        |           |         |          +---------------------------------------- generated candidate scripts, docs, and manifests
  |        |           |         +--------------------------------------------------- ranked risks, provenance slices, owner routes
  |        |           +------------------------------------------------------------- project-native problems, causes, repairs, links
  |        +------------------------------------------------------------------------- files, languages, frameworks, commands, infra, owners
  +---------------------------------------------------------------------------------- source metadata, cache hash, archive path, local root
```

The layers are intentionally file-backed. `patchline repo analyze` can emit a full `analysis-bundle/` so the same material can be inspected by humans, uploaded by CI, reused with `--resume`, redacted with `--redact`, or validated later with `patchline repo offline`.

## Fetch layer

**Purpose.** The fetch layer creates a reproducible local source root from either a GitHub archive or an already-local directory. It records where the source came from before any analyzer interprets it.

**Main commands.**

- `patchline repo fetch --github owner/repo --ref <sha-or-ref> --subpath <path> --out <dir>`
- `patchline repo analyze --github owner/repo --ref <sha-or-ref> --subpath <path> --out <dir>`
- `patchline repo analyze --local <path> --out <dir>`

**Inputs.** Public GitHub repository coordinates, a ref, an optional subpath, or a local project path.

**Outputs.** `source.json`, a content-addressed archive cache entry when a network archive is used, and a local scanned root. Analysis bundles preserve the source metadata copy under `analysis-bundle/source.json`.

**Invariants.**

- The source identity is explicit: repository, ref, subpath, archive hash, cache path, and scanned root.
- Archive downloads are content-addressed and reusable.
- Offline validation can verify cached archive hashes and local scanned roots without network access.

## Inventory layer

**Purpose.** The inventory layer answers "what is in this project slice?" without requiring Patchline-specific formats. It walks the local source tree and identifies languages, SQL files, migration/framework conventions, native commands, tests, infrastructure files, CODEOWNERS routes, and evidence-like exports.

**Main commands.**

- `patchline repo inventory <source-root> --out <dir>`
- `patchline repo analyze ... --stages inventory,...`
- `patchline repo hook pre-commit` and `patchline repo hook pre-push` for changed-file inventory in local git workflows.

**Inputs.** A local source root, including arbitrary repository files.

**Outputs.** Inventory JSON/Markdown with file counts, recognized frameworks, SQL-bearing files, source-embedded SQL, migration commands, native test commands, infrastructure findings, and owner route candidates.

**Invariants.**

- Unknown files are still inventoried rather than discarded.
- Hook modes mirror only changed local files and do not fetch remote data.
- Symlinks and unsafe paths are excluded from scratch-tree mirroring.

## Intake layer

**Purpose.** The intake layer extracts project-native repair evidence from existing data sources. It detects SQL risks, source-embedded queries, deploy and issue exports, Datadog-style exports, OpenTelemetry traces/logs, Jira/Linear issue exports, repair manifests, repair-like scripts, incident-like documents, and identifier links across those sources.

**Main commands.**

- `patchline intake <path> --out <dir>`
- `patchline intake --github owner/repo --ref <sha-or-ref> --subpath <path> --out <dir>`
- `patchline adapt-evidence --adapter <adapter> --input <file> --out <dir>`

**Inputs.** Arbitrary local directories, fetched GitHub slices, or exported observability/issue data.

**Outputs.** Intake JSON/Markdown/SARIF containing discovered files, problem candidates, cause candidates, repair candidates, evidence records, identifier-grounded links, and next commands that can be run immediately.

**Invariants.**

- Findings are deterministic and conservative: links are evidence of shared identifiers, not proof of causality.
- Known export formats enrich reports, but unknown JSON/text still contributes inventory and searchable context.
- Adapter outputs preserve original IDs, timestamps, owners, labels, URLs, and input hashes.

## Baseline layer

**Purpose.** The baseline layer turns inventory and intake facts into ranked risks for the current project state. It is the main "what should a reviewer look at first?" layer.

**Main commands.**

- `patchline repo baseline <analysis-input> --out <dir>`
- `patchline repo analyze ... --stages inventory,baseline`

**Inputs.** Inventory, intake findings, SQL/source facts, schema-evolution facts, framework commands, infrastructure evidence, trace/code links, issue evidence, and repair evidence.

**Outputs.** Baseline JSON/Markdown/SARIF with ranked risks, feature contribution explanations, owner routes, provenance slices, Datalog-style query rows, abstract effects, symbolic obligations, temporal windows, recurrence groups, policy checks, proof-carrying repair summaries, proof-hole minimization, blast-radius estimates, lock/concurrency hazards, privacy/retention hazards, invariant candidates, and trace-to-code links.

**Invariants.**

- Scores are explainable: each ranked risk includes feature contributions and leave-one-feature ablations.
- Abstract interpretation records proof holes when concrete row counts or runtime facts are unavailable.
- CODEOWNERS routes and maintainer triage metadata are advisory reviewer hints, not authorization decisions.

## Proposal layer

**Purpose.** The proposal layer creates candidate repair interventions from deterministic templates or an explicit pluggable generator command. Generated content is never trusted as a fix by itself.

**Main commands.**

- `patchline repo propose <baseline> --out <dir> --no-llm`
- `patchline repo propose <baseline> --out <dir> --llm-command <local-command>`
- `patchline repo analyze ... --stages ...,propose --proposal-kind <kind> --budget files=N,lines=N,tokens=N,changes=N`

**Inputs.** Baseline risks, selected proposal kind, generation budget, optional local generator command, and source context.

**Outputs.** Proposal JSON/Markdown plus generated files such as repair manifests, migration-review notes, rollback plans, dry-run scripts, PR comment bodies, and CI hints. Generated files carry reviewer routes and budget metadata.

**Invariants.**

- Deterministic mode is explicit with `--no-llm`.
- Pluggable generation records generator metadata and determinism assumptions.
- Budgets cap generated files, lines, token estimates, and change counts.
- Generated artifacts are labeled as interventions that must pass compare checks.

## Compare layer

**Purpose.** The compare layer re-analyzes generated intervention files and compares their risk profile to the baseline. This is where Patchline prevents "a plausible patch" from becoming "an accepted repair" without deterministic evidence.

**Main commands.**

- `patchline repo compare <baseline> <proposal> --out <dir>`
- `patchline repo compare ... --run-native-tests`
- `patchline repo analyze ... --stages ...,compare`

**Inputs.** Baseline report, proposal report, generated artifacts, and optionally allowlisted native test commands.

**Outputs.** Compare JSON/Markdown with intervention-loop status, generated-risk findings, new/changed/unchanged finding deltas, native test execution records, log hashes, and CI-friendly report summaries.

**Invariants.**

- Generated files are analyzed as untrusted source inputs.
- Native tests are skipped unless explicitly requested and executed without a shell.
- Compare reports preserve status, exit code, runtime, logs, and hashes for executed checks.

## Deep-analysis layer

**Purpose.** The deep-analysis layer adds research-grade structure on top of baseline and compare reports. It explains why risks were ranked, which evidence is missing, how repair claims could be upgraded, and how repeated patterns recur across a project.

**Main commands.**

- `patchline repo analyze ... --stages ...,deep`
- Baseline and compare commands feed the same deep-analysis fields into their JSON and Markdown outputs.

**Inputs.** Ranked risks, provenance slices, schema-evolution facts, trace/code links, incidents, repair candidates, generated interventions, and compare deltas.

**Outputs.** Minimal cause sets, shared table ancestors, repair lineage, affected outputs, abstract effects, symbolic obligations, temporal windows, recurrence groups, policy obligations, proof-carrying repair summaries, proof-hole minimization, ranking explanations, blast-radius estimates, invariant candidates, and intervention-loop evidence.

**Invariants.**

- The layer preserves uncertainty: open, conditional, and refuted obligations are reported explicitly.
- No concrete execution count is invented when only static evidence is available.
- Query rows and slices are deterministic and capped so reports stay stable across runs.

## Gate layer

**Purpose.** The gate layer turns architecture claims into executable checks. Gates are small scripts and JSON specs that run Patchline against pinned real project slices, verify required artifacts, and fail when a documented behavior regresses.

**Main commands.**

- `make <feature>-gate`
- `patchline gate <report> --policy <policy>`
- CI modes from `patchline repo analyze --ci`

**Inputs.** Pinned public repository slices, generated reports, policies, SARIF, GitLab Code Quality, Bitbucket Code Insights, PR comment bodies, and analysis bundles.

**Outputs.** Pass/fail status, summary JSON, CI artifacts, upload snippets, and focused failure logs.

**Invariants.**

- Real-repo gates use pinned refs so failures are attributable.
- CI artifacts are derived from the same JSON reports humans can inspect.
- Offline and enterprise gates avoid production credentials and can validate cached artifacts under blocked network settings.

## End-to-end artifact contract

`patchline repo analyze` is the reference orchestration command. A complete analysis run can produce:

| Path | Producer layer | Consumer layer |
| --- | --- | --- |
| `source.json` | fetch | inventory, offline |
| `inventory/inventory.json` | inventory | intake, baseline |
| `intake/intake.json` | intake | baseline |
| `baseline/baseline.json` | baseline | proposal, compare, gate |
| `proposal/proposal.json` | proposal | compare, review |
| `proposal/generated/` | proposal | compare, native review |
| `compare/compare.json` | compare | deep-analysis, gate, CI |
| `analysis-bundle/summary.md` | analyze orchestration | humans, CI artifacts |
| `summary.sarif` | analyze orchestration | GitHub code scanning |
| `ci/` | CI mode | GitHub, GitLab, Bitbucket |

The contract is intentionally redundant: JSON is the source of truth, Markdown is for review, SARIF/code-quality outputs are for developer tooling, and bundles are for sharing or offline validation.

## Trust boundaries

Patchline separates facts from interventions.

1. **External input boundary.** Repository archives, local files, and observability exports are untrusted bytes until fetched, hashed, inventoried, and parsed.
2. **Evidence boundary.** Intake facts and adapter events retain source hashes and identifiers so downstream layers can cite where they came from.
3. **Generation boundary.** Proposal output is untrusted, even in deterministic mode; compare must re-run analyzers over generated files.
4. **Execution boundary.** Native tests and database dry-runs require explicit opt-in and local-only execution policies.
5. **Sharing boundary.** Redaction and offline validation create auditable, shareable bundles without assuming access to production services.

## Extension points

New ecosystem support should usually attach at the earliest layer that can represent the evidence honestly:

- Fetch: new source providers, archive formats, and cache metadata.
- Inventory: file classification, framework detection, native command discovery, CODEOWNERS-like ownership metadata.
- Intake: adapters for observability exports, issue trackers, incident documents, repair manifests, and source-embedded SQL.
- Baseline: fact extractors, linkers, ranking features, hazard detectors, invariant miners, and proof-hole reducers.
- Proposal: deterministic templates, local generator adapters, budget policies, and generated artifact renderers.
- Compare: generated-artifact analyzers, native test allowlists, intervention-loop checks, and delta renderers.
- Gate: real-repo matrices, policy thresholds, CI artifact checks, and offline validation scripts.

Prefer adding narrow, deterministic facts over broad heuristics. If a layer cannot prove a stronger claim, it should emit a weaker claim with an explicit proof hole.
