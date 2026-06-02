# Star-growth launch kit

Patchline keeps a **launch kit** for star-growth that bundles the assets a public launch
actually needs — a README hook, a long-form post, a short social thread, a demo script,
and an FAQ — and validates them mechanically so the launch cannot go out with a missing
channel or a social post that exceeds the platform **character limit**.

## Validate before you ship

The worker checks that every required channel is present and non-empty and that each
social post respects its length budget, rejecting any post that runs over the limit.

## Why it matters

A launch fails quietly when a channel is missing or a post is truncated by the platform.
Validating the kit mechanically turns "we're ready to launch" into a checkable claim.

## Reproduce

```
make launch-kit-gate
```
