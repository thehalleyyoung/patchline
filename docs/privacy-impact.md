# Privacy-impact inference

Patchline infers the **privacy impact** of migrations that delete, export, anonymize, or
change the retention of personal data, so that a change touching PII is labeled with its
data-protection consequence rather than treated as an ordinary schema edit.

## Inference

Each operation is joined against a PII classification of the affected column and assigned
an impact:

- **export** of PII → high (personal data leaves the platform);
- **delete** of PII → erasure-relevant (right-to-erasure);
- **anonymization** of PII → mitigating (reduces exposure);
- **retention-policy change** on PII → relevant;
- any operation on a non-PII column → none.

## Why it stays honest

The gate proves a PII export is high impact, a PII delete is erasure-relevant, a PII
**anonymization** is mitigating, a PII retention change is relevant, and a non-PII
operation has no privacy impact — so privacy review focuses exactly on the changes that
touch personal data.

## Reproduce

```
make privacy-impact-gate
```
