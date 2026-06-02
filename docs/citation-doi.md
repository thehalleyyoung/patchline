# Citation file and archival DOI

Patchline provides a citation file and an archival **DOI** so the artifact is formally referenceable.

## How it works

The worker validates the DOI format, checks the citation has title, authors, version, and year, and confirms the cited version matches the release.

## What the gate proves

- The citation is complete with a well-formed DOI.
- A citation with a malformed DOI is rejected.

## Why it matters

A DOI and citation file turn the repository into something researchers can formally cite and build on.

## Reproduce

```
make citation-doi-gate
```
