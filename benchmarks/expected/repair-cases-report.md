# Artifact benchmark report

- dataset_id: `patchline-repair-cases-v1`
- ok: `true`
- hash: `fad8c9074cfdabc88404d204ff02a8454ca540d70b296f6c36998b106fafab7c`
- total: `3`
- passed: `3`
- failed: `0`

| case | type | phase | expected | actual | ok |
| --- | --- | --- | --- | --- | ---: |
| billing-repair-replay | repair | during_repair | verified | verified | true |
| billing-insert-delete-repair | repair | during_repair | verified | verified | true |
| append-only-ledger-manual-rollback | repair | during_repair | cannot_prove | cannot_prove | true |
