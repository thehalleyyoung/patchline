# Release distribution

Patchline supports four installation paths:

1. **Go install**

   ```bash
   go install github.com/thehalleyyoung/patchline/cmd/patchline@latest
   ```

2. **GitHub Releases**

   ```bash
   curl -L -o patchline.tar.gz \
     https://github.com/thehalleyyoung/patchline/releases/download/vX.Y.Z/patchline_Darwin_arm64.tar.gz
   tar -xzf patchline.tar.gz
   ./patchline --help
   ```

3. **Homebrew**

   ```bash
   brew tap thehalleyyoung/patchline
   brew install patchline
   ```

   The release process writes a formula candidate from `packaging/homebrew/patchline.rb`.

4. **Docker**

   ```bash
   docker build -f packaging/docker/Dockerfile -t patchline:local .
   docker run --rm patchline:local --help
   ```

`scripts/package-release.sh` cross-builds Linux and macOS archives, emits installation snippets, copies the Homebrew formula template, and signs sorted checksums with `patchline release checksums`.

```bash
bash scripts/package-release.sh v0.1.0 dist/release-distribution
```

The output includes:

- `patchline_Darwin_amd64.tar.gz`
- `patchline_Darwin_arm64.tar.gz`
- `patchline_Linux_amd64.tar.gz`
- `patchline_Linux_arm64.tar.gz`
- `checksums/checksums.sha256`
- `checksums/checksums.attestation.json`
- `homebrew/patchline.rb`
- `install.md`
- `release-manifest.json`

`make release-distribution-gate` proves the package paths by building the archives, validating the Homebrew/Docker/GitHub Release/Go install instructions, checking signed checksums, and running the packaged host binary against a pinned public `lobsters/lobsters` migration slice.
