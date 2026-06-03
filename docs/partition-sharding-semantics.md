# Partitioning and sharding semantics

Patchline's `db-semantics` report models **partitioning and sharding semantics**
when a migration changes where tenant or partition-owned data is routed. The
detector is intentionally narrower than generic tenant-risk checks: a normal
`WHERE tenant_id = ...` update is not enough. The statement must affect a
route-map or shard-map surface, perform a partition swap/switch/exchange, or
declare a rebalance/reshard movement.

The report records the operation, routing surface, affected scope, tenant key,
partition key, partition name, source and target objects, whether double routing
or rebalance backfill is required, hazards, engine-specific evidence, and
rollout obligations. Supported examples include PostgreSQL `ATTACH/DETACH
PARTITION`, MySQL and Oracle `EXCHANGE PARTITION`, SQL Server `SWITCH
PARTITION`, ClickHouse partition replace/move, BigQuery/Snowflake partitioned
table replacement, tenant route-map updates, and shard rebalances.

Reproduce:

```bash
make partition-sharding-semantics-gate
```
