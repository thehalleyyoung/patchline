# Reviewer-question search

Patchline provides a **reviewer-question search** over the paper's claims, limitations,
artifacts, and roadmap cards so that a reviewer asking a question is answered with the
specific backing entry and its source type rather than a generated guess — and an
**out-of-scope** question returns nothing instead of a hallucinated answer.

## Retrieval

The worker tokenizes a typed corpus (claims, limitations, artifacts, roadmap cards),
removes stopwords, scores each entry against a query by content-token overlap, and
returns the ranked matches with their source type. An empty overlap yields no result.

## Why it stays honest

A retrieval system that always returns something will fabricate answers to questions it
cannot support. The gate proves a determinism question retrieves the determinism claim, a
limitations question retrieves a limitation entry, an artifact question retrieves an
artifact, and an **out-of-scope** query returns zero results.

## Reproduce

```
make reviewer-search-gate
```
