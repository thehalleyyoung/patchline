# Artifact benchmark report

- dataset_id: `patchline-smoke-v1`
- ok: `true`
- hash: `c39bdbaed19f4f3e804a7784e0ed575d22c0da2033fa974fcbdb65008c533bd9`
- total: `4`
- passed: `4`
- failed: `0`

| case | type | phase | expected | actual | ok |
| --- | --- | --- | --- | --- | ---: |
| billing-bad-backfill-migration | migration | pre_deploy | flag | flag | true |
| gitlab-2017-destructive-primary-data | incident | postmortem | flag | flag | true |
| billing-repair-replay | repair | during_repair | verified | verified | true |
| billing-semantic-regression | regression | archive_only | flag | flag | true |
