# Literature positioning

Patchline's novelty is the composition, not a claim that each individual technique is new. It combines operational provenance, relational program semantics, Z3-backed obligations, replayable repair execution, workflow model checking, signed artifacts, and historical incident indexing into one deterministic repair evidence pipeline.

| Prior line of work | What it usually provides | Patchline difference |
| --- | --- | --- |
| Database provenance and why/how provenance | Explains how tuples or outputs depend on inputs. | Connects traces, deploys, migrations, SQL mutations, records, reports, repair manifests, policies, benchmark hashes, and audit ledgers in one incident graph. |
| SQL equivalence and relational verification | Proves query equivalence or containment for focused SQL fragments. | Uses Z3-backed obligations as one layer inside an operator-facing repair workflow with replay, invariants, policy gates, and counterexample artifacts. |
| Program repair and data repair systems | Suggests or synthesizes patches/repairs for constrained bug classes. | Does not synthesize guesses; it validates explicit repair manifests, emits SQL/rollback/transaction plans, and records what is proved, checked, assumed, or refuted. |
| Migration linters | Flag risky migration syntax. | Links migrations to observed traces, row/report damage, repair effects, policy outcomes, strict benchmark hashes, and historical archive buckets. |
| Distributed-systems testing and model checking | Finds schedule/workflow counterexamples. | Applies bounded workflow model checking to incident operations themselves: ingest, explain, approve, dry-run, apply, verify, rollback, audit, archive. |
| Incident-management tooling | Tracks alerts, ownership, timelines, and postmortems. | Produces replayable semantic artifacts: causal certificates, proof obligations, dry-run hashes, invariant checks, signatures, and archive indexes. |
| Observability pipelines | Collect spans, logs, metrics, and events. | Converts exported evidence into a typed repair semantics layer that can be audited without relying on probabilistic clustering or generated explanations. |

The research promise is a falsifiable artifact standard for data/code repair incidents: if an incident is represented as evidence plus a repair transformer, Patchline can say which claims were proved by Z3, checked by bounded replay/model checking, assumed due to missing evidence, unsupported by the current fragment, or refuted by counterexample.

That makes the repo useful for software-engineering research now: it gives concrete JSON schemas and executable commands for studying repair evidence, semantic drift, migration outcomes, recurring incident shapes, and proof-carrying operational reviews over historical data.
