# Artifact benchmark report

- dataset_id: `patchline-public-migrations-v1`
- ok: `true`
- hash: `2ab0dae25d6135703dacd624105ad823a64ce3317a569775811759b534dd3fec`
- total: `5`
- passed: `5`
- failed: `0`

| case | type | phase | expected | actual | ok |
| --- | --- | --- | --- | --- | ---: |
| bytebase-sheet-blob | migration | pre_deploy | flag | flag | true |
| bytebase-replica-heartbeat | migration | pre_deploy | flag | flag | true |
| bytebase-workspace | migration | pre_deploy | flag | flag | true |
| bytebase-drop-sheet-table | migration | pre_deploy | flag | flag | true |
| bytebase-drop-unused-id-columns | migration | pre_deploy | pass | pass | true |
