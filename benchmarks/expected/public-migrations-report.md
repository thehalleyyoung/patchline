# Artifact benchmark report

- dataset_id: `patchline-public-migrations-v1`
- ok: `true`
- hash: `ffd4ff2027aacc6f46cb0c8c18a00278bc84763d4d56d3bf630887bb40cca78e`
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
