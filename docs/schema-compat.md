# Schema registry and protobuf/Avro compatibility

Serialized data outlives the code that wrote it. A producer that evolves a protobuf or Avro schema in
a breaking way can make every previously stored record — and every running consumer — fail to decode.
Patchline ties schema evolution to concrete data-change risk by inspecting the schema formats real
streaming and storage systems use:

| Format | Inspected | Breaking risks flagged |
|--------|-----------|------------------------|
| **protobuf** (`.proto`) | message fields, syntax level | proto2 `required` fields (cannot be removed safely); messages with no `reserved` declarations (tag-reuse hazard) |
| **Avro** (`.avsc`) | record fields | fields without a `default` (break backward/forward compatibility when added or removed) |
| **schema registry** (`buf.yaml`, `buf.gen.yaml`) | compatibility policy | configuration presence so reviewers know a policy governs evolution |

Each risk becomes a searchable `schema_compatibility` fact carrying a `breaking` flag, so schema
evolution is reviewed alongside SQL, NoSQL, and pipeline data-changes.

Guarantees enforced by the gate:

1. Both Avro fields-without-defaults and proto2 required fields are proven against the real
   `apache/avro` repository.
2. The classification and a **no-false-positive** rule (ordinary `.json` config is never treated as
   an Avro schema) are verified by deterministic unit tests.

```
make schema-compat-gate
```

Outputs land in `results/generated/schema-compat/`.
