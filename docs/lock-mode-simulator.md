# Lock-mode simulator

Patchline's **lock-mode simulator** turns normalized migration SQL into an
engine-specific lock/conflict model. It distinguishes PostgreSQL `SHARE` from
`SHARE UPDATE EXCLUSIVE`, MySQL copy-duration metadata locks from instant
metadata barriers, SQL Server offline `Sch-M` from online index phase barriers,
and cloud engines whose documented behavior is metadata-job or transactional
metadata concurrency rather than classic row locks.

Each simulated statement reports the modeled mode, scope, duration class,
reader/writer/DDL conflicts, phase notes, documented behavior references, and a
**containerized smoke** command. The default gate exercises the real CLI for all
supported engines; set `PATCHLINE_RUN_CONTAINERS=1` to rerun the focused Go
fixture checks inside `golang:1.22` with Docker or Podman.

## What the gate proves

- Every supported database engine emits a non-empty lock simulation.
- Online/concurrent paths still expose brief metadata barriers instead of
  claiming "no locks".
- Writer-blocking and non-writer-blocking variants are distinguished for real
  engine/version inputs.
- Every case carries documented behavior evidence and a reproducible
  containerized smoke-test command.

## Reproduce

```bash
make lock-mode-simulator-gate
PATCHLINE_RUN_CONTAINERS=1 make lock-mode-simulator-gate
```
