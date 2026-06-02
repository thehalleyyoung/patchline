# Archive security regression gate

Patchline treats downloaded repository archives as untrusted input. The fetch layer must extract normal public GitHub archives while rejecting or ignoring archive structures that could write outside the extraction root, exhaust disk, or confuse later analysis.

`make archive-security-gate` verifies that contract with focused regression tests and a pinned public repository fetch. The gate covers:

- zip and tar path traversal entries such as `../escape.txt`;
- tar and zip symlink entries that point outside the extraction root;
- malformed `.tar.gz` and `.zip` inputs;
- archive bombs via excessive uncompressed bytes and excessive entry counts;
- successful extraction of valid repository files; and
- content-addressed reuse of a real GitHub archive download.

The production extractor enforces deterministic ceilings on extracted archive entries and uncompressed bytes before writing regular-file content. Symlink entries are skipped rather than materialized, and every output path must remain contained by the requested extraction root.

The real-code portion uses a pinned public repository slice so the security checks are not only synthetic: Patchline downloads, caches, extracts, and scans an ordinary GitHub archive after the malicious cases pass.
