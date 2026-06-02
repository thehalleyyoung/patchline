# Public dataset release with datasheet and DOI

Patchline releases a public dataset with a **datasheet**, license, and DOI passing an ethics review.

## How it works

The worker checks each required release component (datasheet, license, DOI, ethics) is present.

## What the gate proves

- Every release requirement is present.
- A missing requirement is rejected.

## Why it matters

A datasheet, license, DOI, and ethics review together make a dataset citable and responsibly reusable.

## Reproduce

```
make dataset-release-package-gate
```
