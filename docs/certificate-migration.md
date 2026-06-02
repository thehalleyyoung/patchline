# Certificate migration

Patchline ships backward-compatible certificate migration from legacy **PLCI/0** to current PLCI/1 so old proof-carrying verdicts remain checkable across schema revisions.

`make certificate-migration-gate` loads frozen PLCI/0 vectors, verifies their legacy canonical hashes and real repository file digests, maps the old `risk-class` field into PLCI/1 `risk-bps`, re-renders canonical PLCI/1 bytes, and rechecks the migrated certificates with the current parser.

The compatibility corpus covers `safe`, `guarded`, `blocked`, and `unsupported` verdicts plus negative controls for unsafe legacy semantics and unregistered risk classes, so compatibility is proven by executable evidence rather than grandfathered policy.
