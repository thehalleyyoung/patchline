# Certificate interchange language

Patchline now ships **PLCI/1**, a formally specified certificate interchange language for migration-safety verdicts. The ABNF grammar fixes line order, allowed verdicts, evidence references, obligation statuses, file digests, and canonical SHA-256 hashing.

`make certificate-interchange-language-gate` runs executable grammar vectors through four checkers: Go, Rust, Python, and TypeScript. Each checker accepts every valid certificate, rejects every negative control, verifies real `file:` evidence against this repository, and must agree with the other implementations on every vector.

This keeps proof-carrying verdict exchange honest: a certificate is only portable when independent implementations can parse the same bytes, recompute the same hash, and reject the same malformed witnesses.
