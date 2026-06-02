# Generated fixture minimizers

A bug report is only as useful as its reproduction. Patchline ships a fixture minimizer that uses the
real analyzer as an **oracle** and applies **delta-debugging** to reduce any seed input down to the
smallest fixture that still reproduces a destructive data-change finding — for every supported
ecosystem.

For each ecosystem the minimizer:

1. Starts from a seed that contains the target finding plus surrounding noise.
2. Greedily removes lines, re-running the analyzer after each removal and keeping the input only
   while the target fact still fires.
3. Proves the result is **1-minimal**: removing any single remaining line drops the target finding.

| Ecosystem | Target finding | Seed |
|-----------|----------------|------|
| **Cassandra** | destructive `nosql_change` (DROP TABLE/KEYSPACE) | a **real** `laaksomavrick/twitter-go` migration |
| **SQL** | `schema_evolution` drop_table | property fixture |
| **Spark** | destructive `data_pipeline_change` (overwrite) | property fixture |
| **Avro** | breaking `schema_compatibility` (field without default) | property fixture |

Guarantees enforced by the gate:

1. The real Cassandra migration is reduced from its full size to a minimal reproducing fixture, and
   that minimized fixture is independently re-verified to still trigger the destructive fact.
2. Every ecosystem fixture is reduced below its seed size and is 1-minimal.

```
make fixture-minimizer-gate
```

Outputs (including the minimized fixtures) land in `results/generated/fixture-minimizer/`.
