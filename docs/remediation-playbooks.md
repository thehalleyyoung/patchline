# Remediation playbooks

`patchline repo playbook --baseline <baseline-dir>` turns baseline hazards into deterministic maintainer runbooks. It maps high-risk data-change classes such as broad writes, missing transaction boundaries, unsafe idempotency, lock/concurrency hazards, privacy-retention hazards, blast radius, policy obligations, repair-proof gaps, and proof holes to:

- ordered runbook steps,
- rollback decision points before, during, and after execution,
- owner handoffs from baseline CODEOWNERS routes plus explicit fallback roles,
- grounded validation commands from the baseline instead of invented shell commands.

Reproduce the local proof:

```bash
make remediation-playbook-gate
```
