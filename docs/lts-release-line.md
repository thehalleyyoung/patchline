# Long-term-support release line

Patchline maintains an LTS line with **backport**ed security fixes and a clear EOL policy.

## How it works

The worker checks each LTS release receives security backports and has a defined EOL date.

## What the gate proves

- Every LTS release is backported with an EOL.
- A release with no EOL is rejected.

## Why it matters

An LTS line with backports lets conservative adopters stay secure without chasing every release.

## Reproduce

```
make lts-release-line-gate
```
