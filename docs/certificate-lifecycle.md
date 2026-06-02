# Certificate lifecycle

Patchline's PLCI certificates now have a lifecycle that is checkable after interchange, not just parseable at issuance. The lifecycle is intentionally deterministic: normalize before signing, compare normalized witnesses, and replay signed revocation or supersession records through a hash-chain ledger.

`patchline cert normalize <certificate.plci> --root . --out normalized.plci --json` canonicalizes order-insensitive witness sections: evidence rows, obligation rows, and obligation evidence references. It re-parses the normalized bytes with file-digest verification, then reports the normalized canonical hash used by signing, comparison, revocation, and plugfest submissions.

`patchline cert diff <old.plci> <new.plci> --root . --json` produces a semantic obligation diff across current or migrated legacy certificates. Its confidence lattice is explicit: `checked > assumed > unsupported`, while `refuted` remains a counterexample state. Formula, kind, or evidence-reference changes are reported as `changed` instead of guessing logical implication.

`patchline cert revoke-verify <bundle.json> --json` replays signed revocation and supersession records. Each record signs its canonical payload with Ed25519, each ledger entry binds the payload hash, terminal states cannot be changed again, and supersession replacements must be known active certificates.

`patchline cert plugfest --submission submission.json --root . --json` validates an external tool's offline plugfest submission without executing that tool. The validator recomputes the conformance corpus result, normalizer hash, semantic diff summary, revocation replay, and reproducible log hashes in process so standard-scale interoperability remains auditable.

Focused gates:

```bash
make certificate-normalizer-gate
make certificate-semantic-diff-gate
make certificate-revocation-gate
make certificate-plugfest-gate
```
