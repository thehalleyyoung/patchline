# Cross-tool proof-carrying verdict exchange

Patchline now proves cross-tool proof-carrying verdict exchange for three independent migration-safety analyzers: Rails `strong_migrations`, Django migration checks, and Prisma Migrate diagnostics.

`make cross-tool-verdict-exchange-gate` loads native analyzer fixtures from `specs/verdict-exchange/v1`, projects only the fields PLCI/1 promises to preserve, renders a certificate with real `file:` evidence digests, parses it with digest verification enabled, and reconstructs each analyzer-specific projection from the parsed certificate only. The original and reconstructed projections must be equal.

The preserved projection is explicit: analyzer identity, case id, source path, native outcome, normalized PLCI verdict, risk basis points, and obligation evidence/status/message. Native fields outside that projection are intentionally not claimed. The gate also runs negative controls for risk-score drift, digest corruption, missing preserved fields, and unsupported analyzer schemas.
