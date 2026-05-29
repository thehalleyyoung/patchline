# Artifact benchmark report

- dataset_id: `patchline-smoke-v1`
- ok: `true`
- hash: `67396b1fa88d0ab28280298243d026d5c3e056472403c0f4105e49da848a8af9`
- total: `4`
- passed: `4`
- failed: `0`

| case | type | phase | expected | actual | ok |
| --- | --- | --- | --- | --- | ---: |
| billing-bad-backfill-migration | migration | pre_deploy | flag | flag | true |
| gitlab-2017-destructive-primary-data | incident | postmortem | flag | flag | true |
| billing-repair-replay | repair | during_repair | verified | verified | true |
| billing-semantic-regression | regression | archive_only | flag | flag | true |
