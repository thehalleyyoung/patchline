# Replication-lag risk

Patchline's `db-semantics` report now treats replication-lag risk as a
conditional operational proof obligation. It does not assert that a deployment
has read replica, CDC, or event-stream consumers; instead, it links migration
shape to the downstream hazards that must be ruled out before rollout.

The analyzer flags table rewrites, copy-style alters, online schema-change
adapters, table replacement, bulk mutations, and asynchronous mutations. Each
finding names the migration shape, estimated lag class, conditional pipelines,
hazards, engine-specific evidence, mitigations such as declared `max-lag`
throttles, and obligations to bound changed rows or bytes and prove catch-up.

Reproduce:

```bash
make replication-lag-risk-gate
```
