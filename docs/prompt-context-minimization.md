# Prompt context minimization

Patchline proposals use generated artifacts as untrusted repair interventions. The prompt therefore carries only the evidence needed for the selected, budgeted risks instead of copying the full baseline report.

`repo propose` and `repo analyze` write `prompt-context.json`, `prompt.txt`, and `proposal.json` with `prompt_context_minimization` counts. The counts report selected and excluded risks, evidence links, provenance slices, native checks, evidence paths, and excerpt lines. Excluded counts are intentional: they show which baseline context was withheld from the generator while deterministic compare/re-analysis still has access to the full baseline.

The minimization policy is:

1. Select only the highest-ranked risks allowed by the proposal budget.
2. Include only evidence links and provenance slices whose `risk_id` matches those selected risks.
3. Include native checks only when they are attached to selected provenance or mention the selected risk path/table.
4. Include short risk-focused excerpts, preferring the risk statement and operation lines rather than the beginning of a file.
5. Record excluded counts in both the prompt context and proposal report.

Run `make prompt-context-gate` to prove the contract on a focused unit fixture and a pinned public repository slice.
