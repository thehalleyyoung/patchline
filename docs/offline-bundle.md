# Offline runtime-evidence bundle

Reviewers should be able to audit runtime evidence on an air-gapped machine. This gate packages
findings and runtime evidence into a **self-contained** offline bundle that verifies with no
external services.

The bundle contains:

- `findings.json` — real Patchline findings.
- `runtime-evidence.jsonl` — deterministic observed impact per table.
- `INDEX.md` — human-readable entry point.
- `MANIFEST.json` / `MANIFEST.checks` — a sha256 for every member.

Verification contract:

1. **Checksums verify offline** — `shasum -a 256 -c MANIFEST.checks` passes with no network.
2. **Self-contained** — no member references any network endpoint (URLs, hosts, ports).
3. **Manifest complete** — every member is listed, and every listed file exists.
4. **Rebuild deterministic** — rebuilding the bundle yields identical checksums.
5. **Linkage preserved** — every evidence row references a real finding id.

```
make offline-bundle-gate
```

The gate rebuilds and re-verifies the bundle independently. Outputs land in
`results/generated/offline-bundle/`.
