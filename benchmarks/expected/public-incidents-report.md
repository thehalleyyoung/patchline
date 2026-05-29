# Artifact benchmark report

- dataset_id: `patchline-public-incidents-v1`
- ok: `true`
- hash: `fdd6ad5a133f845281e505ec0555fa39d9ef285926bf72d2c03ef09353aa29d5`
- total: `3`
- passed: `3`
- failed: `0`

| case | type | phase | expected | actual | ok |
| --- | --- | --- | --- | --- | ---: |
| gitlab-2017-public-source-observations | incident | postmortem | flag | flag | true |
| github-2018-public-source-observations | incident | postmortem | flag | flag | true |
| public-incident-summary-too-thin | incident | postmortem | insufficient_evidence | insufficient_evidence | true |
