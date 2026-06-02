# Artifact VM/container profile

Patchline's one-command artifact VM/container profile rebuilds public results from pinned public repositories without host-specific assumptions. The profile uses a dedicated artifact Dockerfile plus a host-runnable rebuild script so reviewers can validate the same command path even when a Docker daemon is unavailable.

```bash
make artifact-container-profile-gate
```

The reviewer-facing one-command container path is:

```bash
docker build -f packaging/artifact/Dockerfile -t patchline-artifact-rebuild:local . && docker run --rm patchline-artifact-rebuild:local
```

The profile intentionally avoids production credentials, host database services, host language toolchains, and absolute host paths. Network access is only required to fetch pinned public repository archives used by the capstone public results.

The gate validates:

- the dedicated artifact container recipe and required packages;
- generated profile metadata under `profile/artifact-container-profile.json`;
- explicit host-independence checks under `profile/host-independence.json`;
- the one-command rebuild script under `profile/rebuild-command.sh`;
- regenerated public-code capstone evidence under `public-results/capstone/`;
- public-result thresholds for repositories, ranked risks, generated files, rejected bad output, and evidence artifacts.

If Docker is available, the gate builds and runs the artifact image. Otherwise it verifies the container recipe and executes the same rebuild script on the current host to prove the real public-code path.
