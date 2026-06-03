# Worker loss incident

`worker-b` stopped after leasing `tenant-shard-sweep`. The coordinator reassigned the task to `worker-c`, acquired a deterministic second lease, and accepted only the reassigned result.
