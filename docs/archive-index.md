# Incident archive index

Patchline's archive index turns historical incidents into a deterministic retrieval artifact instead of an ad hoc folder of JSON files. It is useful now because it gives SRE, platform, and research teams stable handles for questions like "which past incidents had this provenance shape?", "which high-risk migrations touched the same table?", and "which repair effects passed policy and benchmark gates?"

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
| Policy decision and failures | Deterministic policy evaluation |
| Benchmark decision and hash | Strict benchmark suite runner |
| Damaged/derived entity counts | Evidence graph |
| Proof-bundle readiness | Presence of dry-run, policy, and benchmark hashes |

The report then emits sorted buckets by evidence shape, migration table, migration risk, repair effect, policy decision, and benchmark decision. Its own `hash` is canonical, so teams can diff archive knowledge over time and attach it to semantic audits or signed attestations.

This is deliberately not a probabilistic clustering system. The index only groups by explicit, recomputed semantic facts; missing or failing inputs surface as command failures rather than guessed categories.
