# A Theory of Risk Classification in Patchline

## Abstract

Patchline calls a change a risk when static evidence from a repository shows that
the change may alter durable state without enough local evidence of scope,
reversibility, idempotency, operational control, or review obligation. The
decision procedure is intentionally not an oracle: it is a deterministic,
evidence-preserving computation over extracted facts, SQL statements, ORM and
migration observations, source windows, and cross-file provenance links. The
output is a ranked set of risk objects, each carrying a kind, severity, score,
factors, identifiers, rationale, stable ID, evidence hash, and follow-on
analyses such as abstract effects, symbolic checks, transaction boundaries,
idempotency classes, lock hazards, privacy hazards, policy checks, repair proof
summaries, and proof-hole minimizations.

This paper describes that procedure from a theoretical and computational
perspective. The core idea is a conservative hazard calculus for data-changing
programs. Patchline first maps project artifacts to canonical facts and
operation observations. It then uses a finite effect lattice to approximate
state change, an additive ranking model to prioritize review, and evidence
joins to distinguish a local syntactic warning from a cross-file operational
risk. The theory is deliberately pragmatic: it chooses decidable,
content-addressed approximations that are explainable and reproducible over
large public repositories, while admitting explicit proof holes when the static
evidence cannot justify a stronger claim.

## 1. Problem statement

Patchline's risk question is not "is this program wrong?" It is:

> Does the repository contain a data-changing operation whose static evidence
> requires human review before it can be treated as safe?

That question is narrower than full program verification and broader than
string matching. A migration, script, ORM method, background job, or support
runbook can be risky even if it is syntactically valid and even if no runtime
failure has been observed. Conversely, a scary-looking token should not be
reported as a high-priority risk unless it is tied to a data-changing operation,
a durable schema or data surface, or an evidence path that makes the warning
reviewable.

Patchline therefore treats risk classification as a decision problem over
evidence:

- **Input:** a repository slice, local directory, fetched public repository, SQL
  file, migration folder, or generated analysis bundle.
- **Extraction:** a finite set of facts, SQL statements, source SQL
  observations, problem candidates, cause candidates, repair candidates, native
  commands, and support files.
- **Decision:** for each operation-like item, decide whether it belongs in the
  review frontier; if so, assign a risk kind, score, severity, evidence links,
  and proof obligations.
- **Output:** deterministic JSON, Markdown, SARIF, and bundle artifacts whose
  hashes and stable IDs are reproducible from the evidence.

The theoretical stance is conservative abstraction. Patchline does not need to
prove that a change will fail. It needs to preserve enough evidence to justify
the weaker claim that a reviewer should inspect the change because safe
execution has not been established locally.

## 2. Evidence model

Patchline represents repository evidence as a typed, content-derived graph.
The important node families are:

- **Facts:** inventory observations with kind, path, stable ID, identifiers,
  properties, and rationale. Facts include files, migration roots, schema
  evolution, source SQL hints, infrastructure, test commands, operational docs,
  repair candidates, and evidence exports.
- **SQL statements:** normalized statements with kind, table, predicate marker,
  fingerprint, risk label, abstract effect, and reasons.
- **Source observations:** ORM, migration-framework, and code-path write
  observations with path, line, operation, table/model identifiers, framework,
  and snippet hash.
- **Intake candidates:** problem, cause, and repair candidates extracted from
  project text, SQL, incidents, notes, repair manifests, and time signals.
- **Provenance slices:** bounded evidence neighborhoods around a risk, keyed by
  strong identifiers such as table, model, framework, or operation rather than
  weak identifiers such as dates.

The evidence graph is not a proof graph in the theorem-prover sense. Its job is
to make review claims navigable and auditable. Identifier joins are used to link
risks with nearby facts, causes, repairs, native commands, and operational
evidence. The baseline implementation explicitly describes provenance slices as
"a navigation aid, not proof of causality." That caveat is central to the
theory: Patchline can say that two artifacts co-reference the same durable
surface; it does not claim causation unless the benchmark or incident ground
truth separately establishes it.

Formally, let:

- `F` be the finite set of extracted facts.
- `S` be the finite set of analyzed SQL statements.
- `O` be the finite set of source or schema write observations.
- `C` be the finite set of intake candidates.
- `I(x)` be the canonical strong identifiers for item `x`.

Patchline constructs evidence links by joining items whose canonical
identifiers intersect:

```text
link(x, f) iff I(x) intersect I(f) is non-empty
```

Weak identifiers such as dates, timestamps, and fields are excluded from
provenance seeding because they tend to produce accidental joins. This preserves
reviewability: a risk should point to concrete paths and identifiers a maintainer
can inspect, not just to coincidental textual overlap.

## 3. The finite effect lattice

The computational core is an abstract interpretation over data-changing
operations. Patchline maps concrete mutations into a small effect lattice:

| Effect | Rank | Meaning |
| --- | ---: | --- |
| `noop` | 0 | no concrete row change |
| `idempotent_update` | 1 | bounded deterministic write whose repeated application is stable |
| `reversible_update` | 2 | bounded write with declared snapshot rollback |
| `replay` | 3 | external replay operation with system-specific semantics |
| `derived_rebuild` | 4 | derived-state rebuild from source records |
| `append_only_external` | 5 | append-only external effect requiring compensating-action semantics |
| `destructive` | 6 | delete or unbounded write that removes or may rewrite state |
| `unknown` | 7 | operation outside the known transfer functions |

The order is a review order, not a moral order. Higher ranks indicate weaker
local static assurance. `unknown` is above `destructive` because an unknown
operation lacks a transfer function; Patchline must keep it in the conservative
frontier until a parser, plugin, manifest, or reviewer supplies more semantics.

The join operator returns the higher-ranked effect. For a sequence of abstract
operations, the summary effect is therefore the worst review obligation in the
sequence:

```text
join(e1, e2) = e1 if rank(e1) >= rank(e2), else e2
summary(E) = fold(join, noop, E)
```

The abstraction records row-count bounds, changed columns, downstream entity
counts, rollback witnesses, and unsupported facts. Its concretization is the set
of concrete executions whose changed rows, columns, downstream impact, and
effect rank are within the summary bounds. When those bounds are missing,
Patchline does not invent precision; it records a proof hole such as "concrete
row count unavailable" or "external operation needs system-specific transfer
function."

## 4. SQL statement classification

Raw SQL is classified by a deterministic lexical analyzer. The analyzer strips
comments, preserves statement boundaries across quoted and dollar-quoted text,
normalizes dialect syntax, replaces literals, tokenizes, and parses enough
statement shape to identify operation kind, table, and obvious predicate
presence. It is not a complete SQL theorem prover. It is a decidable syntactic
front end designed to preserve conservative evidence.

The base rules are:

- `UPDATE` with no predicate is high risk because it can rewrite an entire
  table.
- `UPDATE` with a predicate but without an obvious row key is high risk because
  the predicate may still be broad.
- `UPDATE` with an obvious key predicate is medium risk because it still changes
  persistent data.
- `DELETE` is high risk even when predicate-bounded because it removes rows.
- `DROP` and `TRUNCATE` are high risk because they are destructive.
- `ALTER` is medium risk because schema alteration can invalidate code and
  repair manifests.
- `INSERT` is medium risk because it changes persistent data and may need
  provenance.
- `MERGE` is high risk because it can update or insert many rows.
- recognized `CREATE` is low risk unless dialect rules elevate it.
- unrecognized statement kinds are medium risk because the analyzer lacks a
  transfer function.

Dialect refinements add engine-specific review obligations. PostgreSQL
`CREATE INDEX CONCURRENTLY` can lower lock concern; PostgreSQL non-null default
column additions, MySQL `ALGORITHM=COPY`, MySQL `REPLACE`, SQLite foreign-key
disablement, and other dialect-specific markers can raise risk or add reasons.
The important theoretical point is monotonic conservatism: dialect rules may
add reasons or raise a risk when an engine-specific hazard is visible, but the
analyzer does not suppress durable-state concerns merely because syntax is
valid.

## 5. Source and schema write classification

Repositories often express data changes outside raw SQL. Patchline therefore
extracts code-path observations from ORM calls and migration framework
constructs. A source observation is considered persistent when it is an ORM
write operation such as `update`, `delete`, `insert`, `save`, `merge`,
`persist`, `upsert`, `update_or_create`, `bulk_create`, or `bulk_update`, or
when it is a migration/schema operation such as `create_table`, `drop_table`,
`add_column`, `remove_column`, `drop_column`, `create`, `alter`, or `drop`.

Source-code risks receive factors based on operation family and nearby source
window evidence:

- persistent writes add a base persistent-write factor;
- destructive operations add destructive-code-path weight;
- updates and alters add write-breadth-unknown weight;
- creates and inserts add persistent-create-path weight;
- update/delete windows lacking obvious `where`, `filter`, `id`, or `limit`
  markers add broad-write weight;
- windows lacking transaction, atomic, begin, or commit markers add
  missing-transaction-boundary weight;
- windows lacking idempotency, upsert, uniqueness, or retry-safe markers add
  missing-idempotency weight;
- windows lacking rollback, revert, or dry-run markers add weak-rollback-signal
  weight;
- background, worker, retry, cron, or job markers without idempotency evidence
  add retry-hazard weight.

Schema-evolution facts are classified similarly. Dropping tables or columns is
destructive. Altering or adding columns affects existing records or code paths.
Creating a persistent surface is lower but still reviewable when connected to
durable state.

This gives Patchline a language-agnostic theory: first recover a persistent
operation and its identifiers, then reason about scope, reversibility,
idempotency, and operational context using bounded local evidence.

## 6. Ranking as an explainable additive model

Patchline's baseline risk ranking is an additive score over named factors. Each
factor has a weight and rationale. The score is the sum of weights:

```text
score(r) = sum(weight(f) for f in factors(r))
```

Severity is a thresholding function:

```text
high   iff score >= 90
medium iff 50 <= score < 90
low    iff score < 50
```

Important current factors include:

| Factor | Weight | Meaning |
| --- | ---: | --- |
| `high-risk-sql` | 100 | SQL analyzer classified the statement as high risk |
| `medium-risk-sql` | 30 | SQL analyzer classified the statement as medium risk |
| `intake-problem` | 80 | intake produced a high-severity problem candidate |
| `persistent-write-code-path` | 45 | source or migration framework writes persistent data |
| `destructive-code-path` | 45 | source operation can delete rows or remove data surfaces |
| `destructive-schema-change` | 45 | schema change removes persistent data surface |
| `write-breadth-unknown` | 35 | operation mutates existing records or schema with uncertain breadth |
| `schema-code-path` | 35 | project-native migration or ORM declaration changes persistent schema |
| `persistent-create-path` | 20 | operation creates persistent records or schema |
| `schema-write-breadth` | 20 | schema change can affect existing records or code paths |
| `broad-write` | 20 | nearby evidence suggests an unbounded or broad write |
| `missing-transaction-boundary` | 15 | nearby source lacks obvious transaction boundary |
| `loose-sql` | 10 | SQL was found outside a SQL file and needs context review |
| `operational-context` | 10 | operational evidence shares identifiers with the risk |
| `infrastructure-context` | 10 | infrastructure evidence shares identifiers with the risk |
| `missing-idempotency` | 10 | nearby source lacks idempotency or uniqueness markers |
| `weak-rollback-signal` | 10 | nearby source lacks rollback, revert, or dry-run markers |
| `retry-hazard` | 10 | retry/background signals appear without idempotency evidence |
| `native-check-available` | 5 | native project check is available |
| `source-window-unavailable` | 5 | risk rests on extracted metadata because local source window was unavailable |
| `linked-project-evidence` | up to 10 | project facts share identifiers with this risk |

The model is intentionally simple enough to be audited. Patchline emits ranking
explanations that expand every score into per-feature contributions and
leave-one-feature ablations. For each risk, a reviewer can see the dominant
feature, each feature's share of the score, and whether removing that feature
would change the severity. This is not post-hoc explanation of a black box; it
is faithful explanation of the actual decision procedure.

## 7. Stable identity and reproducibility

Risk reports must remain reviewable across reruns. Patchline assigns two
identity layers:

- an internal risk ID derived from path, statement index, fingerprint, source
  observation hash, or candidate ID;
- a stable risk ID derived from operation family, table, canonical identifiers,
  factor names, and provenance stages.

The stable ID deliberately avoids raw path-only identity when stronger semantic
identifiers are available. This allows reviewers to recognize the same table or
operation family across generated bundles, public-repo reruns, and staged
pipeline outputs. Patchline also emits an evidence hash over the stable ID,
table, operation family, and severity. The hash is not a proof of correctness;
it is a tamper-evident handle for the classified evidence state.

## 8. The decision algorithm

At a high level, baseline classification is:

```text
Input: inventory inv, intake report rep, facts F
Output: baseline report B

1. Index facts by canonical identifier.
2. For each SQL statement in rep.SQL:
     if SQL risk is high or medium:
       create a BaselineRisk from statement kind, table, reasons, and score factors.
3. For each high-severity intake problem:
     create a BaselineRisk from problem kind, table, identifiers, and rationale.
4. For each persistent source SQL or ORM observation:
     classify operation family and local source-window markers.
5. For each schema-evolution fact:
     classify schema operation family and persistence effect.
6. Deduplicate risks and sort by score, path, and ID.
7. Build ranking explanations.
8. Link risks to facts through shared strong identifiers.
9. Build provenance slices, then assign stable IDs and evidence hashes.
10. Run follow-on analyses: abstract effects, symbolic checks, temporal windows,
    recurrences, transaction boundaries, idempotency classes, lock hazards,
    privacy hazards, infrastructure findings, invariants, trace links, blast
    radius estimates, policy checks, repair proofs, and proof minimizations.
11. Write JSON, Markdown, SARIF, summary counts, and content hash.
```

Computationally, the procedure is finite and deterministic. The main costs are
linear scans of files and facts, identifier-index joins, bounded sorting, and
bounded report truncation for large explanation families. The classifier avoids
whole-program symbolic execution. Instead, it computes a conservative review
frontier with enough paths, identifiers, and proof obligations for humans or
CI gates to decide the next action.

## 9. Why a risk is not a confirmed bug

Patchline's theory separates three levels of claim:

1. **Risk:** static evidence says a durable-state operation requires review.
2. **Hazard:** a follow-on analysis identifies a specific failure mode such as
   missing transaction boundary, non-idempotency, lock contention, privacy or
   retention exposure, weak rollback, broad blast radius, or missing policy
   obligation.
3. **Confirmed bug or incident:** an external ground-truth benchmark, public
   incident record, or adjudicated case establishes that the risk corresponds
   to a real failure.

Most baseline entries are risks, not confirmed bugs. This distinction is
essential. A high-risk `DELETE` with no rollback evidence is a valid review
frontier item even if it never caused an outage. A public incident benchmark is
a stronger claim because the ground truth ties the pattern to a known failure.

## 10. Symbolic checks and proof obligations

After ranking, Patchline projects risks into abstract operations and asks
symbolic questions over the approximation:

- **Idempotency:** does repeated application satisfy `T(T(state)) == T(state)`?
- **Reversibility:** is there an inverse or rollback witness such that
  `T^-1(T(state)) == state`?
- **Frame:** are untouched columns preserved?
- **Scope:** is the row or table scope bounded?

The statuses are `pass`, `warn`, or `fail`. Destructive and unknown effects
fail idempotency and reversibility unless stronger evidence exists. Missing row
counts, missing changed-column sets, external replay semantics, append-only
effects, and absent rollback evidence become proof holes rather than silent
successes.

This gives Patchline a proof-carrying style without claiming full formal
verification. Every stronger conclusion must be backed by a witness in the
local evidence. When the witness is absent, the report says what is missing and
which minimal artifacts could upgrade the risk from uncertain to checked.

## 11. Transaction, idempotency, lock, and privacy analyses

Patchline's follow-on analyses are specialized classifiers over the same
evidence graph.

**Transaction boundary inference** searches nearby text for explicit
transaction blocks, atomic markers, SQL begin/commit/rollback, savepoints, and
framework-specific markers such as Rails transactions, Django `transaction.atomic`,
SQLAlchemy `session.begin`, TypeORM transaction managers, Prisma `$transaction`,
and GORM transactions. It classifies boundaries as explicit, partial, or
missing with confidence derived from available text.

**Idempotency classification** combines symbolic checks with textual markers
such as idempotency notes, upserts, conflict guards, uniqueness guards,
existence guards, checkpoints, dry runs, and scoped keys. Destructive operations
without guards are non-idempotent. Guarded operations are distinct from proven
operations; a guard is useful evidence but not necessarily a proof.

**Lock and concurrency hazards** search risks and support files for blocking
DDL, explicit locks, index builds, non-online index creation, table rewrites,
broad writes, job contention, and blast-radius reads. Mitigations include
concurrent indexes, online DDL, batching, skip-locked/nowait behavior,
transaction boundaries, advisory locks, and lock or statement timeouts.

**Privacy and retention hazards** search destructive, export, anonymization,
retention, backfill, repair, and sensitive-identifier paths for broad deletes,
anonymization changes, export scripts, sensitive identifiers, retention-policy
markers, rollback gaps, and broad updates. Mitigations include scoped
predicates, snapshots, dry runs, anonymized output, retention windows, approval
or audit evidence, and rollback evidence.

These analyses do not create the initial risk frontier by themselves. They
refine what kind of review the risk requires.

## 12. Policy checks and repair proof summaries

Patchline converts selected risk classes into policy obligations. The current
policy family requires combinations of guard, rollback, approval, dry-run, and
test evidence before a generated or manual repair should be trusted. Missing
critical obligations fail; missing non-critical obligations warn. Satisfied
obligations are recorded with evidence from provenance slices and symbolic
checks.

Repair proof summaries then ask whether a linked repair source satisfies scope,
frame, rollback, and proof obligations. A repair can be checked, conditional,
open, or refuted. This is especially important for generated interventions:
Patchline can generate tests, guards, instrumentation, repair candidates, and
explanations, but the compare step still treats generated artifacts as
untrusted until coverage, quarantine, and deterministic checks pass.

## 13. Computational properties

Patchline's risk classifier has several useful computational properties:

- **Decidability:** every classifier operates over finite extracted artifacts,
  bounded source windows, finite token streams, and finite identifier joins.
- **Determinism:** given the same inputs, dialect settings, and generated
  artifacts, baseline reports are sorted and hashed deterministically.
- **Monotonic review conservatism:** adding evidence can add risks, increase
  scores, add links, or satisfy obligations. Missing evidence does not produce
  a false proof of safety.
- **Explainability:** every score is the sum of named factors, and every
  ranking explanation is derived directly from those factors.
- **Auditability:** every risk points to paths, identifiers, rationale,
  evidence links, and next commands rather than only to a scalar score.
- **Bounded imprecision:** unsupported semantics are exposed as proof holes,
  not hidden behind success-shaped defaults.
- **Language extensibility:** new extractors can feed the same persistent
  operation, identifier, and effect interfaces without changing the review
  theory.

The tradeoff is intentional incompleteness. Patchline will miss risks that do
not leave recognizable static evidence, and it may flag risks that a domain
expert can later discharge with external context. The classifier is designed to
make those cases explicit and reviewable, not to pretend they do not exist.

## 14. Soundness boundary

The useful soundness statement is conditional:

> If extraction finds a persistent operation and the operation matches one of
> the implemented transfer or scoring rules, Patchline will place that operation
> in the review frontier with an explanation at least as conservative as the
> matched rule.

This is not a guarantee that every risky program operation in the repository is
found. The boundary is limited by parsers, source windows, repository contents,
generated artifacts, and available operational evidence. It is also not a
guarantee that every risk is a bug. A risk is a request for review backed by
static evidence; a bug is a stronger empirical or semantic claim.

Patchline's theory is therefore best understood as a computable abstraction for
data-change review:

```text
concrete repository behavior
    -> extracted facts and operation observations
    -> effect lattice and additive review score
    -> evidence-linked risk frontier
    -> symbolic checks and policy obligations
    -> reviewer action, generated intervention, or explicit proof hole
```

That pipeline is what lets Patchline run across raw SQL, Flask/SQLite examples,
Django and Alembic migrations, Rails migrations, Prisma, TypeORM, dbt models,
Kubernetes data-service manifests, Protobuf examples, MongoDB Prisma schemas,
and benchmark incident artifacts while keeping the same central decision rule:
call something a risk when durable-state change is visible and local evidence
does not yet prove that scope, rollback, idempotency, concurrency, privacy, and
policy obligations are satisfied.

## 15. How to read a risk report as a theoretical object

A baseline risk entry should be read as:

```text
Risk r = (operation, durable surface, abstract effect, score, severity,
          evidence links, proof obligations)
```

The `kind` and `table` identify the operation and durable surface. The score and
severity identify where it sits in the review frontier. The factors are the
derivation tree for that score. The identifiers and evidence links define the
neighborhood of repository evidence. The abstract effects and symbolic checks
state which semantic properties are proven, warned, or failed. The policy and
repair proof summaries state what must be supplied before a change can be
treated as safe or a generated intervention can be trusted.

In short, Patchline calls something a risk when it can construct this tuple with
enough evidence to make a conservative, reproducible review claim. It does not
require omniscience; it requires a durable operation, a finite approximation,
and an explicit account of what remains unproven.

## Reproduce the paper's implementation checks

Run:

```bash
make theory-risk-paper-gate
```

The gate builds Patchline, runs inventory/intake/baseline on `demos/billing`,
checks that baseline output includes risks, ranking explanations, abstract
effects, symbolic checks, policy checks, and proof-hole minimizations, and
verifies that the paper's central constants and rule names still appear in the
implementation.
