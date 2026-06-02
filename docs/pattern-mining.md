# Cross-repository pattern mining

Patchline mines recurring migration **failure mode**s across a corpus of repositories so
that a class of mistake appearing in many projects is promoted to a named **recurring
pattern**, while a one-off failure seen in a single repository is not — focusing
remediation effort on systemic problems rather than incidents.

## Mining

The worker counts the number of distinct repositories exhibiting each failure mode, marks
a mode as recurring once it crosses a minimum-repository threshold, and ranks recurring
patterns by prevalence.

## Why it stays honest

The gate proves a failure mode present in three repositories is promoted to a recurring
pattern, a failure mode present in only one repository is excluded, and the ranking is
ordered by repository count — so "recurring" reflects real cross-project prevalence, not
a single noisy incident.

## Reproduce

```
make pattern-mining-gate
```
