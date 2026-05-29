# Incident archive index

Patchline's archive index turns historical incidents into a deterministic retrieval artifact instead of an ad hoc folder of JSON files. It is useful now because it gives SRE, platform, and research teams stable handles for questions like "which past incidents had this provenance shape?", "which high-risk migrations touched the same table?", "which repair effects passed policy and benchmark gates?", "which verified repairs later resembled another archived incident?", and "which new transition is a semantic regression against prior incident knowledge?"

Run the bundled corpus:

```bash
go run ./cmd/patchline archive-index examples/archive/bad-migration-corpus.json
go run ./cmd/patchline archive-index examples/archive/bad-migration-corpus.json --json
```

An archive spec is a list of incidents, where each incident points at existing evidence, migration, repair, policy, and benchmark artifacts:

```json
{
  "version": "patchline.archive-index/v1",
  "name": "bad-migration-corpus",
  "incidents": [
    {
      "id": "billing-bad-backfill",
      "evidence": "../incidents/bad-migration.jsonl",
      "migration": "../../demos/billing/migrations/002_bad_backfill.sql",
      "repair": "../repairs/repair-bad-invoice-backfill.json",
      "policy": "../policies/review-required.json",
      "benchmark": "../benchmarks/strict-migration-corpus.json"
    },
    {
      "id": "billing-corrective-repair",
      "evidence": "../incidents/bad-migration.jsonl",
      "migration": "../../demos/billing/migrations/003_corrective_repair.sql",
      "repair": "../repairs/repair-bad-invoice-backfill.json",
      "policy": "../policies/review-required.json",
      "benchmark": "../benchmarks/strict-migration-corpus.json"
    }
  ]
}
```

For each incident, Patchline recomputes:

| Field | Source |
| --- | --- |
| Evidence hash and shape hash | JSONL ingestion plus provenance graph shape |
| Migration hash, tables, and maximum risk | SQL migration analyzer |
| Repair hash and repair effect | Manifest replay plus abstract effect summary |
| Repair outcome hashes | Dry-run replay hash, generated/applied SQL hash, verification hash, rollback availability, and later recurrence candidates |
| Semantic regressions | Ordered comparisons from later incidents to prior same-shape, shared high-risk table, or damaged-derived-report evidence, with learned invariant candidates |
| Policy decision and failures | Deterministic policy evaluation |
| Benchmark decision and hash | Strict benchmark suite runner |
| Damaged/derived entity counts | Evidence graph |
| Proof-bundle readiness | Presence of dry-run, policy, and benchmark hashes |

The report then emits sorted buckets by evidence shape, migration table, migration risk, repair effect, policy decision, and benchmark decision. Its own `hash` is canonical, so teams can diff archive knowledge over time and attach it to semantic audits or signed attestations.

This is deliberately not a probabilistic clustering system. The index only groups by explicit, recomputed semantic facts; missing or failing inputs surface as command failures rather than guessed categories.

## Deterministic historical queries

`archive-query` turns the same recomputed archive facts into immediately reviewable historical knowledge:

```bash
go run ./cmd/patchline archive-query examples/archive/bad-migration-corpus.json
go run ./cmd/patchline archive-query examples/archive/bad-migration-corpus.json broad-updates --json
go run ./cmd/patchline archive-query examples/archive/bad-migration-corpus.json damaged-reports --json
go run ./cmd/patchline archive-query examples/archive/bad-migration-corpus.json missing-rollback --json
go run ./cmd/patchline archive-query examples/archive/bad-migration-corpus.json repair-outcomes --json
go run ./cmd/patchline repair-outcomes examples/archive/bad-migration-corpus.json --json
go run ./cmd/patchline archive-query examples/archive/bad-migration-corpus.json semantic-regressions --json
go run ./cmd/patchline semantic-regressions examples/archive/bad-migration-corpus.json --json
```

The default bundled corpus currently answers:

| Query | Grounded result |
| --- | --- |
| Which migrations caused broad updates? | `billing-bad-backfill` contains a high-risk `update invoices` statement fingerprinted as `update invoices set total_cents = ? where status = ?`. |
| Which reports derived from damaged rows? | `report:monthly_revenue` is reachable through `derived_into` evidence from damaged billing records. |
| Which repairs lacked rollback? | None in the default corpus; the result is an empty list and the query hash still records that checked fact. |
| Which repair outcomes are verified and recurrence-linked? | `billing-bad-backfill` and `billing-corrective-repair` both carry dry-run, SQL, verification, rollback, and outcome hashes; the first records `billing-corrective-repair` as a later same-shape/table recurrence candidate. |
| Which later transition semantically regressed against prior history? | `billing-corrective-repair` is compared against `billing-bad-backfill` by same semantic shape, shared high-risk table evidence, and damaged-derived-report recurrence, producing concrete learned invariant candidates rather than a prose-only postmortem lesson. |

These queries are deterministic joins over evidence graph structure, migration analyzer output, repair rollback metadata, policy decisions, and benchmark hashes. They are useful on historical archives because they answer operational questions without inventing hidden classifications: every answer carries the incident id, artifact paths, hashes, and canonical query hash.
