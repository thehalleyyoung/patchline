# Artifact benchmark report

- dataset_id: `patchline-smoke-v1`
- ok: `true`
- hash: `ac63990a6962437c44088d9a944207d65bcafa1b4348ba1437caf9b249e0a1cc`
- total: `4`
- passed: `4`
- failed: `0`

| case | type | phase | expected | actual | ok |
| --- | --- | --- | --- | --- | ---: |
| billing-bad-backfill-migration | migration | pre_deploy | flag | flag | true |
| gitlab-2017-destructive-primary-data | incident | postmortem | flag | flag | true |
| billing-repair-replay | repair | during_repair | verified | verified | true |
| billing-semantic-regression | regression | archive_only | flag | flag | true |
