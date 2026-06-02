# PLCI/1 certificate interchange language

PLCI/1 is Patchline's dependency-free certificate interchange language for migration-safety verdicts. The ABNF grammar fixes field order, enums, evidence references, file digests, verdict semantics, and canonical SHA-256 hashing so independent implementations can exchange the same certificate without trusting the producer.

The executable conformance vectors live under `vectors/valid` and `vectors/invalid`. Every checker must accept every valid vector, reject every invalid vector, verify real `file:` evidence digests against this repository, and agree on every accept/reject decision.

Run:

```bash
make certificate-interchange-language-gate
```
