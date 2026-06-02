# Monorepo package-boundary detection

Real repositories are increasingly **monorepos**: many independently deployable packages share one
checkout. A destructive migration or unguarded write lives inside *one* package, but a naive scanner
reports it against the repository root, which is useless for routing and ownership.

Patchline's inventory now detects **package boundaries** from build-system manifests and records, per
package, its build `system`, its `path`, and the `manifest` that declares it. Supported systems:

- **Bazel** — `WORKSPACE`/`MODULE.bazel` workspace plus per-package `BUILD.bazel` files.
- **Pants** — `pants.toml` workspace.
- **Nx** — `nx.json` workspace plus per-package `project.json`/`package.json`.
- **Turborepo** — `turbo.json` workspace plus per-package `package.json`.
- **Maven** — per-module `pom.xml`.
- **Gradle** — `settings.gradle` plus per-subproject `build.gradle`.
- **Go workspaces** — `go.work` plus per-module `go.mod`.

A **no-false-positive** rule keeps this precise: a per-directory `BUILD.bazel` or `package.json`
becomes a boundary only when a workspace marker confirms the monorepo, so incidental files named
`BUILD` or `package.json` do not invent packages.

Guarantees enforced by the gate:

1. Boundaries are detected on a **real Turborepo monorepo** (`vercel/turbo` `examples/basic`),
   including the workspace root and the expected packages.
2. The full ecosystem matrix (Bazel, Pants, Nx, Turborepo, Maven, Gradle, Go workspaces) and the
   no-false-positive rule are verified by deterministic unit tests.

```
make monorepo-boundary-gate
```

Outputs land in `results/generated/monorepo-boundary/`.
