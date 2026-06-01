# Artifact benchmark report

- dataset_id: `patchline-negative-v1`
- ok: `true`
- hash: `807a45b284870ccb0fc93a215c3b3adb644a174e0d23bb1b66a56113f9f37d36`
- total: `5`
- passed: `5`
- failed: `0`

| case | type | phase | expected | actual | ok |
| --- | --- | --- | --- | --- | ---: |
| unsupported-procedural-sql | migration | pre_deploy | unsupported_fragment | unsupported_fragment | true |
| insufficient-public-incident-evidence | incident | postmortem | insufficient_evidence | insufficient_evidence | true |
| predeploy-postmortem-leakage | incident | pre_deploy | cannot_prove | cannot_prove | true |
| non-replayable-missing-backup | repair | during_repair | cannot_prove | cannot_prove | true |
| safe-scoped-nonrecurrence | regression | archive_only | pass | pass | true |
