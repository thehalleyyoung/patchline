# Artifact benchmark report

- dataset_id: `patchline-smoke-v1`
- ok: `true`
- hash: `b5a3619fb0ab8e62cfcd42808f7a1acd227b127a5d95511e1863294bf2aaf469`
- total: `4`
- passed: `4`
- failed: `0`

| case | type | phase | expected | actual | ok |
| --- | --- | --- | --- | --- | ---: |
| billing-bad-backfill-migration | migration | pre_deploy | flag | flag | true |
| gitlab-2017-destructive-primary-data | incident | postmortem | flag | flag | true |
| billing-repair-replay | repair | during_repair | verified | verified | true |
| billing-semantic-regression | regression | archive_only | flag | flag | true |
