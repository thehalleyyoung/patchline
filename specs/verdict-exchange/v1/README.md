# Cross-tool verdict exchange v1

This corpus proves cross-tool proof-carrying verdict exchange for three independent migration-safety analyzers: Rails `strong_migrations`, Django migration checks, and Prisma Migrate diagnostics.

Each native fixture is projected into the fields PLCI/1 is expected to preserve: analyzer identity, case id, source path, native outcome, normalized verdict, risk basis points, and obligation evidence. The gate renders a PLCI/1 certificate from that projection, parses it with real file digest verification, reconstructs the analyzer-specific projection from the parsed certificate only, and requires equality with the original projection. Negative controls mutate preserved risk, corrupt a digest, omit a preserved field, and use an unsupported analyzer schema.
