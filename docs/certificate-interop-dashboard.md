# Certificate interop dashboard

Patchline ships a certificate interop dashboard for PLCI/1 so the frozen standards-body corpus is not trusted only because one implementation accepts it.

`make certificate-interop-dashboard-gate` rebuilds a byte-preserving vector view of `specs/certificate-conformance/v1/corpus.json`, reruns the Go, Rust, Python, and TypeScript checkers with `--root .` against real Patchline evidence files, verifies the signed reference outputs, and writes:

- `dashboard.json`: checker-by-checker drift deltas for acceptance, rejection, verdict, risk, canonical hash, certificate ID, certificate bytes, and negative-control error text.
- `dashboard.md`: a public table showing whether each checker still agrees with the frozen signed references.
- `gate-summary.json`: the compact release-gate result.

The gate also tampers one checker report and requires the dashboard to fail with a `canonical_sha256` drift, proving the published dashboard catches cross-implementation disagreement instead of merely regenerating a passing report.
