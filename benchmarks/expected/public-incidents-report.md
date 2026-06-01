# Artifact benchmark report

- dataset_id: `patchline-public-incidents-v1`
- ok: `true`
- hash: `083a953c0354f94a23ebe8409e486311b238c1ede9a8fb27b180cb157239aefc`
- total: `3`
- passed: `3`
- failed: `0`

| case | type | phase | expected | actual | ok |
| --- | --- | --- | --- | --- | ---: |
| gitlab-2017-public-source-observations | incident | postmortem | flag | flag | true |
| github-2018-public-source-observations | incident | postmortem | flag | flag | true |
| public-incident-summary-too-thin | incident | postmortem | insufficient_evidence | insufficient_evidence | true |
