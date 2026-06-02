# 1.0 release-readiness dossier

Patchline's 1.0 **release-readiness dossier** is a capstone that certifies release-
readiness by checking, for a representative set of named capabilities, that the full
**evidence chain** exists on disk — an example specification, a worker script, a gate
validator, a documentation page, a Makefile target, and a README mention that invokes the
gate — so the 1.0 claim is backed by a mechanical audit of every artifact rather than a
hand-written changelog.

## Audit the whole chain

The worker resolves each capability to its six expected artifacts, marks a capability
certified only when all are present and wired, and reports any gap. A phantom capability
with none of its artifacts is reported uncertified.

## Why it matters

"Ready for 1.0" is a claim that should be checkable. By auditing the six-artifact chain
for every capability, release-readiness becomes a reproducible fact.

## Reproduce

```
make dossier-gate
```
