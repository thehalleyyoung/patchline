# False-positive adjudication (blinded, multi-rater)

Credible false-positive numbers need blinded examples and more than one rater. This gate
runs a **blinded, multi-rater adjudication** of Patchline findings:

- **Blinding.** Each finding is reduced to neutral evidence fields (operation family,
  presence of independent danger evidence, presence of a policy-control gap). Severity and
  score are withheld so raters cannot simply echo the ranking.
- **Three reviewers of differing strictness.** Each reviewer scores the same blinded
  evidence — independent incident/repair/recurrence evidence, a missing guard/rollback on a
  failing policy, and a destructive operation family — into a signal score, then applies its
  own acceptance threshold (lenient, moderate, strict). This mirrors real review panels where
  reviewers disagree on where to draw the line.
- **Agreement + adjudication.** The workflow reports pairwise **Cohen's kappa**, mean kappa,
  the three-way full-agreement rate, and the **majority-adjudicated false-positive rate**.

```
make fp-adjudication-gate
```

It downloads real repositories, blinds every finding, runs the three raters, and computes
the statistics. The gate fails unless mean kappa clears the configured floor (raters are
genuinely independent, not degenerate), the blinded set leaks no severity/score, and the
majority verdicts partition cleanly into true/false positive.

Outputs (`results/generated/fp-adjudication/`):

- `blinded-examples.jsonl` — the severity/score-free adjudication set.
- `rater-labels.jsonl` — per-example votes and the majority verdict.
- `fp-adjudication.json` / `.md` — rater rates, kappa table, and the adjudicated FP rate.

This makes the project's false-positive claims reproducible and auditable instead of
anecdotal.
