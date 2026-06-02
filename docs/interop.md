# SARIF-style cross-tool interop

Patchline supports **cross-tool interop** by exporting its migration findings into a
**SARIF**-style interchange document and importing them back, proving the mapping is
lossless for the fields that matter — rule id, severity level, message, and file
location — so a finding produced by Patchline can flow into any SARIF-consuming dashboard
and return without distortion.

## Round-trip fidelity

The worker emits a SARIF-like document from native findings, parses it back into native
form, and checks the round-trip is field-for-field identical. A malformed interchange
document that omits a required field (a missing `ruleId`) is rejected by the validator.

## Why it matters

Interoperability claims are worthless if the export drops fields. A field-for-field
round-trip check turns "SARIF-compatible" into a verified property.

## Reproduce

```
make interop-gate
```
