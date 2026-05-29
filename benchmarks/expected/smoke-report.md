# Artifact benchmark report

- dataset_id: `patchline-smoke-v1`
- ok: `true`
- hash: `2626447e24ed733a4662c01a03e780780751c2194854bf19acaaf2f1eba08e90`
- total: `4`
- passed: `4`
- failed: `0`

| case | type | phase | expected | actual | ok |
| --- | --- | --- | --- | --- | ---: |
| billing-bad-backfill-migration | migration | pre_deploy | flag | flag | true |
| gitlab-2017-destructive-primary-data | incident | postmortem | flag | flag | true |
| billing-repair-replay | repair | during_repair | verified | verified | true |
| billing-semantic-regression | regression | archive_only | flag | flag | true |
