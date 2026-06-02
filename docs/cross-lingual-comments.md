# Cross-lingual code-comment analysis

Patchline adds cross-lingual comment analysis improving extraction on **non-English** codebases.

## How it works

The worker checks extraction succeeds for each supported comment language.

## What the gate proves

- Extraction works for every supported language.
- A failed-language extraction is rejected.

## Why it matters

Cross-lingual extraction broadens coverage to the large share of code commented in non-English languages.

## Reproduce

```
make cross-lingual-comments-gate
```
