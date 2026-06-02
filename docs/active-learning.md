# Active-learning queue

Patchline maintains an **active-learning queue** that decides which migration examples a
human should label next by prioritizing the most informative ones — those whose model
confidence sits closest to the decision **boundary** — while excluding examples that are
already labeled and deprioritizing confidently-classified ones.

## Spend review where it counts

The worker scores each unlabeled example by boundary proximity, drops already-labeled
examples, and returns a top-k queue ordered by informativeness. A confidently-classified
example ranks below a boundary one, and an already-labeled example never appears.

## Why it matters

Human review is the scarcest resource. Querying the examples nearest the decision boundary
reduces uncertainty fastest per label, so the queue maximizes information per unit of
effort.

## Reproduce

```
make active-learning-gate
```
