# NoSQL migration and change detection

Data loss is not a relational-only problem. NoSQL stores carry just as much risk, but their
destructive operations look nothing like SQL `DROP TABLE`. Patchline's inventory now recognizes
schema-change and data-migration operations against five NoSQL engines, scanning each file only when
an engine-specific **signal** confirms the target so prose and unrelated code are not misclassified:

| Engine | Signals | Destructive operations flagged |
|--------|---------|--------------------------------|
| **MongoDB** | `db.<coll>`, `.collection(`, migrate-mongo | `drop`, `dropDatabase`, `deleteMany`/`deleteOne`, `$unset`, `renameCollection` |
| **Cassandra** | `.cql`, `KEYSPACE`, `CREATE TABLE` | `DROP KEYSPACE`, `DROP TABLE`, `TRUNCATE`, `ALTER … DROP` |
| **Elasticsearch** | `_bulk`, `_mapping`, `_delete_by_query` | index `DELETE`, `_delete_by_query`, bulk delete |
| **Redis** | `redis-cli`, `.redis` | `FLUSHALL`, `FLUSHDB`, `DEL`, `RENAME` |
| **DynamoDB** | `dynamodb` | `delete-table`, batch `DeleteRequest` |

Every matched operation becomes a `nosql_change` fact with `engine`, `operation`, and a
`destructive` flag, so destructive NoSQL changes surface as searchable evidence alongside SQL
migration risks.

Guarantees enforced by the gate:

1. **Cassandra** destructive-change detection is proven against the real `laaksomavrick/twitter-go`
   repository, including `DROP KEYSPACE` and `DROP TABLE`.
2. The full **five-engine matrix** and a **no-false-positive** rule for prose are verified by
   deterministic unit tests.

```
make nosql-change-gate
```

Outputs land in `results/generated/nosql-change/`.
