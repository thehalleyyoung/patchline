# Compatibility gate

`make compatibility-gate` checks Patchline against minimal platform assumptions rather than a contributor's full workstation.

The gate validates:

| Surface | Check |
| --- | --- |
| macOS | Cross-builds the CLI for the configured Darwin target and runs the host binary when it matches the current machine. |
| Linux | Cross-builds the CLI for the configured Linux target with `CGO_ENABLED=0`. |
| Real code | Runs `repo analyze` on a pinned public repository slice with the host toolchain and verifies findings, generated interventions, and compare loops. |
| Minimal tools | Requires only `go`, `git`, `bash`, `jq`, and `make` for the non-container path. |
| Container | Validates the Dockerfile/devcontainer recipe and required packages; if a Docker daemon is available, the gate also builds and runs the documented container smoke command. |

The default spec is `examples/compatibility-gate.json`. It currently proves a Darwin build, a Linux build, the documented container recipe, and a real Lobsters migration slice.
