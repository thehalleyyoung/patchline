# Contributor recognition

Patchline contributor recognition is generated from public proof gates so acknowledgements stay tied to real project improvements. It recognizes work on new real-repo slices, ecosystem parsers, false-positive reductions, and artifact improvements.

```bash
make contributor-recognition-gate
```

The generator reruns proof gates for Awesome Patchline examples, rejected generated-code examples, and reviewability examples. It emits:

- `contributor-recognition.json`: machine-readable contributors, badges, scores, categories, and proof links.
- `leaderboard.md`: public recognition table.
- `cards/*.md`: one contributor card per recognized contribution.
- `proofs/*`: raw public-code proof outputs and logs used for scoring.

The gate rejects recognition entries that lack a proof gate, category, badge, positive score, or regenerated evidence.
